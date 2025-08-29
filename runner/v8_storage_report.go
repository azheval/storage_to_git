package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"storage_to_git/models"
)

func processReports(logger *slog.Logger, projectPath string, storageUsers []models.UserMapping, project *models.Project) ([]*models.Report, error) {
	var reports []*models.Report

	files, err := filepath.Glob(filepath.Join(projectPath, "*.report"))
	if err != nil {
		return nil, fmt.Errorf("error finding report files: %v", err)
	}

	if len(files) == 0 {
		logger.Warn("Report files not found", "path", projectPath)
		return reports, nil
	}

	for _, file := range files {
		report, err := parseReportFile(logger, file, storageUsers, project)
		if err != nil {
			logger.Error("Error parsing report", "file", file, "error", err)
			continue
		}
		reports = append(reports, report)
	}

	return reports, nil
}

func parseReportFile(logger *slog.Logger, filePath string, storageUsers []models.UserMapping, project *models.Project) (*models.Report, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open report file: %v", err)
	}
	defer file.Close()

	report := &models.Report{
		FileName: filepath.Base(filePath),
	}
	scanner := bufio.NewScanner(file)
	var currentVersion *models.ReportVersion

	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(strings.ToLower(line), "отчет по версиям хранилища:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				path := strings.TrimSpace(parts[1])
				report.StoragePath = path
				logger.Debug("Storage path found", "path", path)
			}
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Дата отчета:") {
			dateStr := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			report.ReportDate, err = parseDate(dateStr)
			if err != nil {
				return nil, fmt.Errorf("error parsing report date: %v", err)
			}
		} else if strings.HasPrefix(line, "Время отчета:") {
			timeStr := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			report.ReportTime, err = parseTime(timeStr)
			if err != nil {
				return nil, fmt.Errorf("error parsing report time: %v", err)
			}
		}

		// Парсим информацию о версиях
		if strings.HasPrefix(line, "Версия:") && !strings.Contains(line, "Версия конфигурации") {
			if currentVersion != nil {
				report.Versions = append(report.Versions, *currentVersion)
			}

			version := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			currentVersion = &models.ReportVersion{
				Version:     version,
				FileName:    report.FileName,
				StoragePath: report.StoragePath,
			}

			if strings.HasSuffix(report.FileName, "cf.report") {
				storage := findStorage(logger, project, report.StoragePath)
				if storage != nil {
					currentVersion.Storage = *storage
				}
			} else {
				extension := findExtension(logger, project, report.StoragePath)
				if extension != nil {
					currentVersion.Extension = *extension
				}
			}

		} else if currentVersion != nil {
			if strings.HasPrefix(line, "Версия конфигурации:") {
				currentVersion.ConfigVersion = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			} else if strings.HasPrefix(line, "Пользователь:") {
				currentVersion.User = findUserMapping(storageUsers, strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
			} else if strings.HasPrefix(line, "Дата создания:") {
				dateStr := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				currentVersion.CreationDate, err = parseDate(dateStr)
				if err != nil {
					return nil, fmt.Errorf("error parsing creation date: %v", err)
				}
			} else if strings.HasPrefix(line, "Время создания:") {
				timeStr := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				currentVersion.CreationTime, err = parseTime(timeStr)
				if err != nil {
					return nil, fmt.Errorf("error parsing creation time: %v", err)
				}
			} else if strings.HasPrefix(line, "Комментарий:") {
				comment := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				// Читаем многострочный комментарий
				for scanner.Scan() {
					nextLine := strings.TrimSpace(scanner.Text())
					if nextLine == "" || strings.Contains(nextLine, ":") {
						break
					}
					comment += "\n" + nextLine
				}
				currentVersion.Comment = comment
			} else if strings.Contains(line, "Добавлены") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					if _, err := fmt.Sscanf(parts[1], "%d", &currentVersion.AddedCount); err != nil {
						logger.Warn("error reading AddedCount", "line", line, "error", err)
					}
				}
			} else if strings.Contains(line, "Изменены") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					if _, err := fmt.Sscanf(parts[1], "%d", &currentVersion.ChangedCount); err != nil {
						logger.Warn("error reading ChangedCount", "line", line, "error", err)
					}
				}
			}
		}
	}

	if currentVersion != nil {
		report.Versions = append(report.Versions, *currentVersion)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning file: %v", err)
	}

	return report, nil
}

func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("02.01.2006", dateStr)
}

func parseTime(timeStr string) (time.Time, error) {
	return time.Parse("15:04:05", timeStr)
}

func getAllVersions(reports []*models.Report) []models.ReportVersion {
	var allVersions []models.ReportVersion
	for _, report := range reports {
		allVersions = append(allVersions, report.Versions...)
	}
	return allVersions
}

func sortVersionsByCreation(versions []models.ReportVersion) {
	sort.Slice(versions, func(i, j int) bool {
		if !versions[i].CreationDate.Equal(versions[j].CreationDate) {
			return versions[i].CreationDate.Before(versions[j].CreationDate)
		}
		return versions[i].CreationTime.Before(versions[j].CreationTime)
	})
}

func saveVersionsConfig(projectPath string, config models.VersionMap) error {
	file, err := os.Create(projectPath)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(config)
	if err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}

	return nil
}

func updateVersionsConfig(logger *slog.Logger, config models.VersionMap, version models.ReportVersion) models.VersionMap {
	updatedConfig := make(models.VersionMap)
	for k, v := range config {
		updatedConfig[k] = v
	}

	fileKey := getFileKey(version.FileName)
	versionNum, err := strconv.Atoi(version.Version)
	if err != nil {
		logger.Error("error parsing version", "version", version.Version, "error", err)
		return updatedConfig
	}

	updatedConfig[fileKey] = versionNum

	return updatedConfig
}

func filterVersionsByConfig(logger *slog.Logger, versions []models.ReportVersion, config models.VersionMap) []models.ReportVersion {
	var filtered []models.ReportVersion

	for _, version := range versions {
		fileKey := getFileKey(version.FileName)
		lastVersion, exists := config[fileKey]
		versionNum, err := strconv.Atoi(version.Version)
		if err != nil {
			logger.Error("error parsing version", "version", version.Version, "error", err)
			continue
		}

		if !exists || versionNum > lastVersion {
			filtered = append(filtered, version)
		}
	}

	return filtered
}

func getFileKey(fileName string) string {
	baseName := filepath.Base(fileName)
	return strings.TrimSuffix(baseName, ".report")
}

func readVersionsConfig(projectPath string) (models.VersionMap, error) {
	config := make(models.VersionMap)

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return config, nil
	}

	file, err := os.Open(projectPath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON: %v", err)
	}

	return config, nil
}

func findUserMapping(storageUsers []models.UserMapping, user string) models.UserMapping {
	for _, mapping := range storageUsers {
		if mapping.StorageUser == user {
			return mapping
		}
	}

	for _, mapping := range storageUsers {
		if mapping.StorageUser == "default" {
			return mapping
		}
	}

	return models.UserMapping{}
}

func findStorage(logger *slog.Logger, project *models.Project, reportStoragePath string) *models.Storage {
	if project.Storage != nil {
		logger.Debug("compare paths", "config_path", project.Storage.StoragePath, "report_path", reportStoragePath)
		if ComparePaths(logger, project.Storage.StoragePath, reportStoragePath) {
			logger.Debug("paths match")
			return project.Storage
		}
	}
	return nil
}

func findExtension(logger *slog.Logger, project *models.Project, reportStoragePath string) *models.Extension {
	for i := range project.Extensions {
		logger.Debug("compare paths for extension", "config_path", project.Extensions[i].StoragePath, "report_path", reportStoragePath)
		if ComparePaths(logger, project.Extensions[i].StoragePath, reportStoragePath) {
			logger.Debug("paths match")
			return &project.Extensions[i]
		}
	}
	return nil
}

func ComparePaths(logger *slog.Logger, path1, path2 string) bool {
	normalized1 := filepath.ToSlash(path1)
	normalized2 := filepath.ToSlash(path2)

	if runtime.GOOS == "windows" {
		normalized1 = strings.ToLower(normalized1)
		normalized2 = strings.ToLower(normalized2)
	}

	clean1 := filepath.Clean(normalized1)
	clean2 := filepath.Clean(normalized2)

	logger.Debug("compare paths", "path1_cleaned", clean1, "path2_cleaned", clean2, "result", clean1 == clean2)

	return clean1 == clean2
}