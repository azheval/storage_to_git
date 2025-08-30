package runner

import (
	"bufio"
	"log/slog"
	"os"
)

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
