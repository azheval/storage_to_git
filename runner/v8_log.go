package runner

import (
	"bufio"
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var utf8bom = []byte{0xEF, 0xBB, 0xBF}

func Log1C(logger *slog.Logger, v8LogFile string) {
	file, err := os.Open(v8LogFile)
	if err != nil {
		logger.Debug("1C log file not found, skipping", "path", v8LogFile)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		logger.Debug(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		logger.Error("Error reading 1C log file", "error", err)
	}
}

func getDumpFilePath(logFilePath string) string {
	return logFilePath[:len(logFilePath)-len(filepath.Ext(logFilePath))] + ".dump"
}

func CheckForErrors(logger *slog.Logger, logFilePath string) bool {
	errorFilePath := getDumpFilePath(logFilePath)
	if _, err := os.Stat(errorFilePath); os.IsNotExist(err) {
		return false
	}

	content, err := os.ReadFile(errorFilePath)
	if err != nil {
		logger.Error("Failed to read dump file", "path", errorFilePath, "error", err)
		return true
	}

	content = bytes.TrimPrefix(content, utf8bom)

	trimmedContent := strings.TrimSpace(string(content))

	if trimmedContent == "0" {
		return false
	}

	logger.Error("Error detected in dump file or dump file is empty/invalid", "path", errorFilePath, "content", trimmedContent)
	return true
}
