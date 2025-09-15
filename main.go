package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"storage_to_git/models"
	"storage_to_git/runner"
	"storage_to_git/storage"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	projectCancelFuncs    = make(map[string]context.CancelFunc)
	projectExecutionLocks = make(map[string]*sync.Mutex)
	projectMutex          sync.Mutex
)

var version = "development"

func main() {
	versionFlag := flag.Bool("version", false, "Print application version and exit")
	configFile := flag.String("config", "", "Path to the configuration file")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s\nwritten by azheval <azheval@gmail.com>\n", version)
		os.Exit(0)
	}

	var configPath string
	if *configFile != "" {
		configPath = *configFile
	} else {
		exePath, err := os.Executable()
		if err != nil {
			println("Failed to get executable path: " + err.Error())
			os.Exit(1)
		}
		configPath = filepath.Join(filepath.Dir(exePath), "config.json")
	}

	jsonFile, err := os.Open(configPath)
	if err != nil {
		println("Failed to open config file: " + err.Error())
		os.Exit(1)
	}

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		println("Failed to read config file: " + err.Error())
		os.Exit(1)
	}
	jsonFile.Close()

	var config models.Config
	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		println("Failed to unmarshal config JSON: " + err.Error())
		os.Exit(1)
	}

	if !filepath.IsAbs(config.AppLogDir) {
		config.AppLogDir = filepath.Join(filepath.Dir(configPath), config.AppLogDir)
	}

	logsDirPath := config.AppLogDir
	if _, err := os.Stat(logsDirPath); os.IsNotExist(err) {
		if err := os.Mkdir(logsDirPath, os.ModePerm); err != nil {
			println("Failed to create logs directory: " + err.Error())
			os.Exit(1)
		}
	}

	logfileName := fmt.Sprintf("storage_to_git-%s.log", time.Now().Format("2006-01-02"))
	logPath := filepath.Join(logsDirPath, logfileName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		println("Failed to open log file: " + err.Error())
		os.Exit(1)
	}

	var logLevel slog.Level
	switch config.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	replaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		if a.Value.Kind() == slog.KindString {
			a.Value = slog.StringValue(strings.ReplaceAll(a.Value.String(), "\\", "\\"))
		}
		return a
	}

	logger := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: replaceAttr,
	}))
	slog.SetDefault(logger)

	updateProjects(&config)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Failed to create file watcher", "error", err)
		os.Exit(1)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					slog.Info("Config file modified. Reloading projects...")
					// Re-read config and update projects
					jsonFile, err := os.Open(configPath)
					if err != nil {
						slog.Error("Failed to open config file", "error", err)
						continue
					}
					byteValue, err := io.ReadAll(jsonFile)
					if err != nil {
						slog.Error("Failed to read config file", "error", err)
						jsonFile.Close()
						continue
					}
					jsonFile.Close()

					var newConfig models.Config
					err = json.Unmarshal(byteValue, &newConfig)
					if err != nil {
						slog.Error("Failed to unmarshal config JSON", "error", err)
						continue
					}
					updateProjects(&newConfig)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("File watcher error", "error", err)
			}
		}
	}()

	err = watcher.Add(configPath)
	if err != nil {
		slog.Error("Failed to add config file to watcher", "error", err)
		os.Exit(1)
	}

	// Block main goroutine forever.
	<-make(chan struct{})
}

func updateProjects(config *models.Config) {
	projectMutex.Lock()
	defer projectMutex.Unlock()

	activeProjects := make(map[string]bool)

	for i := range config.Projects {
		project := &config.Projects[i]
		if !project.Enabled {
			if cancel, exists := projectCancelFuncs[project.Name]; exists {
				slog.Info("Stopping disabled project", "project", project.Name)
				cancel()
				delete(projectCancelFuncs, project.Name)
				delete(projectExecutionLocks, project.Name)
			}
			continue
		}

		activeProjects[project.Name] = true

		if _, exists := projectCancelFuncs[project.Name]; !exists {
			projectExecutionLocks[project.Name] = &sync.Mutex{}
			ctx, cancel := context.WithCancel(context.Background())
			projectLogger := slog.Default().With("project", project.Name)
			ctx = models.WithLogger(ctx, projectLogger)
			projectCancelFuncs[project.Name] = cancel
			go runProject(ctx, config, project)
		}
	}

	for name, cancel := range projectCancelFuncs {
		if !activeProjects[name] {
			slog.Info("Stopping removed project", "project", name)
			cancel()
			delete(projectCancelFuncs, name)
			delete(projectExecutionLocks, name)
		}
	}
}

func runProject(ctx context.Context, config *models.Config, project *models.Project) {
	logger := models.FromContext(ctx)

	projectMutex.Lock()
	lock, ok := projectExecutionLocks[project.Name]
	projectMutex.Unlock()

	if !ok {
		logger.Error("Could not find execution lock for project")
		return
	}

	logger.Info("Starting project")

	// Initial run
	lock.Lock()
	processProject(ctx, config, project)
	lock.Unlock()

	if !project.ScheduleEnabled {
		logger.Info("Project is not scheduled for repeated runs.")
		return
	}

	scheduleDuration, err := time.ParseDuration(project.Schedule)
	if err != nil {
		logger.Error("Invalid schedule duration for project", "schedule", project.Schedule, "error", err)
		return
	}

	ticker := time.NewTicker(scheduleDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if lock.TryLock() {
				logger.Info("Ticker fired, starting scheduled run.")
				go func() {
					defer lock.Unlock()
					processProject(ctx, config, project)
				}()
			} else {
				logger.Info("Skipping scheduled run: project is still running.")
			}
		case <-ctx.Done():
			logger.Info("Stopping project")
			return
		}
	}
}

func processProject(ctx context.Context, config *models.Config, project *models.Project) {
	logger := models.FromContext(ctx)
	logger.Info("Processing project")

	versions, err := storage.LoadOrInitVersions(filepath.Join(filepath.Dir(project.ProjectDataPath), project.VersionsFilePath), *project)
	if err != nil {
		logger.Error("Error loading or initializing versions for project", "error", err)
		return
	}

	logger.Info("Successfully loaded versions for project", "versions", versions)

	users, err := storage.LoadUserMappings(filepath.Join(filepath.Dir(project.ProjectDataPath), project.UsersFilePath))
	if err != nil {
		logger.Error("Error loading user mappings for project", "error", err)
		return
	}
	logger.Debug("Successfully loaded users for project", "users", users)

	runner.Run(ctx, config, project, users)
}