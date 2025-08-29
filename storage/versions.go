package storage

import (
	"encoding/json"
	"errors"
	//"fmt"
	"os"
	//"os/exec"
	//"path/filepath"
	"storage_to_git/models"
)

// LoadOrInitVersions loads a version map from a file, or creates and saves a new one if it doesn't exist.
func LoadOrInitVersions(filePath string, project models.Project) (models.VersionMap, error) {
	_, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		// File does not exist, create a new one
		versions := make(models.VersionMap)

		// Check if storage section exists
		if project.Storage != nil {
			versions["cf"] = 0
		}

		// Add extensions
		for _, ext := range project.Extensions {
			versions[ext.ExtensionName] = 0
		}

		err = SaveVersions(filePath, versions)
		if err != nil {
			return nil, err
		}
		return versions, nil
	}

	// File exists, load it
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var versions models.VersionMap
	err = json.Unmarshal(file, &versions)
	if err != nil {
		return nil, err
	}

	return versions, nil
}

// SaveVersions saves a version map to a file.
func SaveVersions(filePath string, versions models.VersionMap) error {
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}
