package models

import (
	"context"
	"log/slog"
)

type LoggerKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey{}, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(LoggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

type Config struct {
	LogLevel    string    `json:"log_level"`
	Catalog1cv8 string    `json:"catalog_1cv8"`
	Projects    []Project `json:"projects"`
}

type Project struct {
	Name                          string      `json:"project"`
	Catalog1cv8                   string      `json:"catalog_1cv8,omitempty"`
	Enabled                       bool        `json:"enabled"`
	Schedule                      string      `json:"schedule"`
	ScheduleEnabled               bool        `json:"schedule_enabled"`
	ProjectDataPath               string      `json:"project_data_path"`
	UsersFilePath                 string      `json:"users_file_path"`
	VersionsFilePath              string      `json:"versions_file_path"`
	V8LogFilePath                 string      `json:"v8_log_file_path"`
	GitRepositoryPath             string      `json:"git_repository_path"`
	GitRemoteUrl                  string      `json:"git_remote_url"`
	BranchName                    string      `json:"branch_name"`
	GitPushEnabled                bool        `json:"git_push_enabled"`
	GitPushTimingAfterEachCommit  bool        `json:"git_push_timing_after_each_commit"`
	InfoBase                      InfoBase    `json:"infobase"`
	Storage                       *Storage    `json:"storage,omitempty"`
	Extensions                    []Extension `json:"extensions,omitempty"`
}

type InfoBase struct {
	InfoBasePath     string `json:"infobase_path"`
	InfoBaseUser     string `json:"infobase_user"`
	InfoBasePassword string `json:"infobase_password"`
}

type Storage struct {
	StoragePath     string `json:"storage_path"`
	StorageUser     string `json:"storage_user"`
	StoragePassword string `json:"storage_password"`
	GitRepositoryPath   string `json:"git_repository_path"`
}

type Extension struct {
	ExtensionName            string `json:"extension_name"`
	StoragePath     string `json:"storage_path"`
	StorageUser     string `json:"storage_user"`
	StoragePassword string `json:"storage_password"`
	GitRepositoryPath        string `json:"git_repository_path"`
}