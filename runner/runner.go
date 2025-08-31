package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"storage_to_git/git"
	"storage_to_git/models"
)


func Run(ctx context.Context, config *models.Config, project *models.Project, storageUsers []models.UserMapping) {
	logger := models.FromContext(ctx)

	v8path := config.Catalog1cv8
	if project.Catalog1cv8 != "" {
		v8path = project.Catalog1cv8
	}
	v8files := NewV8Files(v8path)

	logger.Info("Running commands for project", "1cv8_path", v8files.ThickClient)

	ibUser := &IBUser{
		Name:     project.InfoBase.InfoBaseUser,
		Password: project.InfoBase.InfoBasePassword,
	}

	infobase := &Infobase{
		Path:           project.InfoBase.InfoBasePath,
		User:           ibUser,
	}

	logFilePath := filepath.Join(filepath.Dir(project.ProjectDataPath), project.V8LogFilePath)
	dumpFilePath := getDumpFilePath(logFilePath)

	if project.Storage != nil {

		storageUser := &StorageUser{
			Name:     project.Storage.StorageUser,
			Password: project.Storage.StoragePassword,
		}

		storage := &Storage{
			Path:           project.Storage.StoragePath,
			User:           storageUser,
		}

		reportFilePath := filepath.Join(filepath.Dir(project.ProjectDataPath), "cf.report")

		commandLine := fmt.Sprintf("DESIGNER /DisableStartupDialogs %s %s /ConfigurationRepositoryReport %q -ReportFormat txt /OUT %q /DumpResult %q", infobase.ConnectionString(), storage.ConnectionString(), reportFilePath, logFilePath, dumpFilePath)
		logger.Info("Executing configuration repository report command")
		args := splitCommandLine(commandLine)
		_, err, hasError := executeCommand(logger, v8files.ThickClient, logFilePath, args...)
		if err != nil || hasError {
			logger.Error("Command execution failed", "error", err)
			return
		}
	}

	for _, ext := range project.Extensions {

		extensionUser := &StorageUser{
			Name:     ext.StorageUser,
			Password: ext.StoragePassword,
		}

		extension := &Storage{
			Path:           ext.StoragePath,
			User:           extensionUser,
		}

		reportFilePath := filepath.Join(filepath.Dir(project.ProjectDataPath), fmt.Sprintf("%s.report", ext.ExtensionName))

		commandLine := fmt.Sprintf("DESIGNER /DisableStartupDialogs %s %s /ConfigurationRepositoryReport %q -ReportFormat txt -Extension %s /OUT %q /DumpResult %q", infobase.ConnectionString(), extension.ConnectionString(), reportFilePath, ext.ExtensionName, logFilePath, dumpFilePath)
		logger.Info("Executing extension repository report command", "extension", ext.ExtensionName)
		args := splitCommandLine(commandLine)
		_, err, hasError := executeCommand(logger, v8files.ThickClient, logFilePath, args...)
		if err != nil || hasError {
			logger.Error("Command execution failed", "error", err)
			return
		}
	}

	versionFilePath := filepath.Join(project.ProjectDataPath, project.VersionsFilePath)
	versionMap, err := readVersionsConfig(versionFilePath)
	if err != nil {
		logger.Error("Failed to read versions config", "error", err)
		return
	}

	reports, err := processReports(logger, project.ProjectDataPath, storageUsers, project)
	if err != nil {
		logger.Error("Failed to process reports", "error", err)
		return
	}

	allVersions := getAllVersions(reports)

	filteredVersions := filterVersionsByConfig(logger, allVersions, versionMap)

	sortVersionsByCreation(filteredVersions)

	mainRepo, err := git.NewRepository(logger, project.GitRepositoryPath, project.GitRemoteUrl)
	if err != nil {
		logger.Error("Failed to initialize main git repository", "path", project.GitRepositoryPath, "error", err)
		return
	}

	if project.BranchName != "" {
		err = mainRepo.Checkout(logger, project.BranchName)
		if err != nil {
			logger.Error("Failed to checkout branch", "branch", project.BranchName, "error", err)
			return
		}
	} else {
		logger.Warn("Branch name is not specified for the project")
	}

	pushNeeded := false

	for _, version := range filteredVersions {
		logger.Info("Processing version", "version", version.Version)

		commitDate := time.Date(
			version.CreationDate.Year(),
			version.CreationDate.Month(),
			version.CreationDate.Day(),
			version.CreationTime.Hour(),
			version.CreationTime.Minute(),
			version.CreationTime.Second(),
			0,
			version.CreationDate.Location(),
		)

		commitSuccess := false

		if version.Storage.StoragePath != "" {
			logger.Info("Processing main configuration version", "version", version.Version)

			storageUser := &StorageUser{
				Name:     version.Storage.StorageUser,
				Password: version.Storage.StoragePassword,
			}

			storage := &Storage{
				Path:           version.Storage.StoragePath,
				User:           storageUser,
			}

			commandLine := fmt.Sprintf("DESIGNER /DisableStartupDialogs %s %s /ConfigurationRepositoryUnbindCfg -force /OUT %q /DumpResult %q", infobase.ConnectionString(), storage.ConnectionString(), logFilePath, dumpFilePath)
			logger.Info("Executing unbind command")
			args := splitCommandLine(commandLine)
			_, err, hasError := executeCommand(logger, v8files.ThickClient, logFilePath, args...)
			if err != nil || hasError {
				logger.Error("Command unbind failed", "error", err)
				return
			}

			commandLine = fmt.Sprintf("DESIGNER /DisableStartupDialogs %s %s /ConfigurationRepositoryUpdateCfg -v %s -force /OUT %q /DumpResult %q", infobase.ConnectionString(), storage.ConnectionString(), version.Version, logFilePath, dumpFilePath)
			logger.Info("Executing update command")
			args = splitCommandLine(commandLine)
			_, err, hasError = executeCommand(logger, v8files.ThickClient, logFilePath, args...)
			if err != nil || hasError {
				logger.Error("Command update failed", "error", err)
				return
			}

			gitDumpPath := filepath.Join(project.GitRepositoryPath, version.Storage.GitRepositoryPath)
			
			if err := os.MkdirAll(gitDumpPath, os.ModePerm); err != nil {
				logger.Error("Failed to create directory for git repository", "path", gitDumpPath, "error", err)
				continue
			}

			dumpFlags := ""
			dumpInfoPath := filepath.Join(gitDumpPath, "ConfigDumpInfo.xml")
			if _, err := os.Stat(dumpInfoPath); err == nil {
				dumpFlags = "-update -force"
			} else if !os.IsNotExist(err) {
				logger.Error("Error checking ConfigDumpInfo.xml", "path", dumpInfoPath, "error", err)
			}

			commandLine = fmt.Sprintf("DESIGNER /DisableStartupDialogs %s /DumpConfigToFiles %q %s /OUT %q /DumpResult %q", infobase.ConnectionString(), gitDumpPath, dumpFlags, logFilePath, dumpFilePath)
			logger.Info("Executing dump to files command")
			args = splitCommandLine(commandLine)
			_, err, hasError = executeCommand(logger, v8files.ThickClient, logFilePath, args...)
			if err != nil || hasError {
				logger.Error("Command dump to files failed", "error", err)
				return
			}

			logger.Info("Executing git commit")
			currentBranch, err := mainRepo.GetCurrentBranch()
			if err != nil {
				logger.Error("Failed to get current branch", "error", err)
			} else if currentBranch != project.BranchName {
				logger.Error("Wrong branch before commit", "expected", project.BranchName, "current", currentBranch)
			} else {
				commitMade, err := mainRepo.Commit(logger, version.User.GitUser, version.User.GitEmail, version.Comment, commitDate)
				if err != nil {
					logger.Error("Git commit failed", "error", err)
				} else {
					commitSuccess = commitMade
				}
			}

		} else if version.Extension.StoragePath != "" {
			logger.Info("Processing extension version", "extension", version.Extension.ExtensionName, "version", version.Version)

			storageUser := &StorageUser{
				Name:     version.Extension.StorageUser,
				Password: version.Extension.StoragePassword,
			}

			storage := &Storage{
				Path:           version.Extension.StoragePath,
				User:           storageUser,
			}

			extensionName := version.Extension.ExtensionName

			commandLine := fmt.Sprintf("DESIGNER /DisableStartupDialogs %s %s /ConfigurationRepositoryUnbindCfg -force -Extension %s /OUT %q /DumpResult %q", infobase.ConnectionString(), storage.ConnectionString(), extensionName, logFilePath, dumpFilePath)
			logger.Info("Executing unbind command for extension")
			args := splitCommandLine(commandLine)
			_, err, hasError := executeCommand(logger, v8files.ThickClient, logFilePath, args...)
			if err != nil || hasError {
				logger.Error("Command unbind failed", "error", err)
				return
			}

			commandLine = fmt.Sprintf("DESIGNER /DisableStartupStartupDialogs %s %s /ConfigurationRepositoryUpdateCfg -v %s -force -Extension %s /OUT %q /DumpResult %q", infobase.ConnectionString(), storage.ConnectionString(), version.Version, extensionName, logFilePath, dumpFilePath)
			logger.Info("Executing update command for extension")
			args = splitCommandLine(commandLine)
			_, err, hasError = executeCommand(logger, v8files.ThickClient, logFilePath, args...)
			if err != nil || hasError {
				logger.Error("Command update failed", "error", err)
				return
			}

			gitDumpPath := filepath.Join(project.GitRepositoryPath, version.Extension.GitRepositoryPath, extensionName)
			
			if err := os.MkdirAll(gitDumpPath, os.ModePerm); err != nil {
				logger.Error("Failed to create directory for git repository", "path", gitDumpPath, "error", err)
				continue
			}

			dumpFlags := ""
			dumpInfoPath := filepath.Join(gitDumpPath, "ConfigDumpInfo.xml")
			if _, err := os.Stat(dumpInfoPath); err == nil {
				dumpFlags = "-update -force"
			} else if !os.IsNotExist(err) {
				logger.Error("Error checking ConfigDumpInfo.xml", "path", dumpInfoPath, "error", err)
			}

			commandLine = fmt.Sprintf("DESIGNER /DisableStartupDialogs %s /DumpConfigToFiles %q %s -Extension %s /OUT %q /DumpResult %q", infobase.ConnectionString(), gitDumpPath, dumpFlags, extensionName, logFilePath, dumpFilePath)
			logger.Info("Executing dump to files command for extension")
			args = splitCommandLine(commandLine)
			_, err, hasError = executeCommand(logger, v8files.ThickClient, logFilePath, args...)
			if err != nil || hasError {
				logger.Error("Command dump to files failed", "error", err)
				return
			}

			logger.Info("Executing git commands")
			currentBranch, err := mainRepo.GetCurrentBranch()
			if err != nil {
				logger.Error("Failed to get current branch", "error", err)
			} else if currentBranch != project.BranchName {
				logger.Error("Wrong branch before commit", "expected", project.BranchName, "current", currentBranch)
			} else {
				commitMade, err := mainRepo.Commit(logger, version.User.GitUser, version.User.GitEmail, version.Comment, commitDate)
				if err != nil {
					logger.Error("Git commit failed", "error", err)
				} else {
					commitSuccess = commitMade
				}
			}
		}

		if commitSuccess && project.GitPushEnabled {
			pushNeeded = true
			if project.GitPushTimingAfterEachCommit {
				logger.Info("Pushing after each commit", "repo", mainRepo.Path)
				if err := mainRepo.Push(logger, "origin", project.BranchName); err != nil {
					logger.Error("Git push failed", "error", err)
				}
			}
		}

		versionMap = updateVersionsConfig(logger, versionMap, version)

		err = saveVersionsConfig(versionFilePath, versionMap)
		if err != nil {
			logger.Error("Failed to save versions config", "error", err)
			return
		}
	}

	if pushNeeded && project.GitPushEnabled && !project.GitPushTimingAfterEachCommit {
		logger.Info("Pushing all modified repositories at the end", "repo", mainRepo.Path)
		if err := mainRepo.Push(logger, "origin", project.BranchName); err != nil {
			logger.Error("Git push failed", "repo", mainRepo.Path, "error", err)
		}
	}

	logger.Info("Runner completed successfully")
}

func splitCommandLine(s string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false

	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case unicode.IsSpace(r) && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func executeCommand(logger *slog.Logger, name, logFilePath string, arg ...string) ([]byte, error, bool) {
	cmd := exec.Command(name, arg...)

	logger.Debug("Executing command", "command", cmd.String())

	output, err := cmd.CombinedOutput()
	Log1C(logger, logFilePath)

	hasError := CheckForErrors(logger, logFilePath)
	if err != nil {
		logger.Error("Failed to execute command", "error", err, "output", string(output))
		return output, fmt.Errorf("command execution failed: %w", err), hasError
	}

	logger.Info("Command executed successfully", "output", string(output))
	return output, nil, hasError
}
