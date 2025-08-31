package storage

import (
	"encoding/json"
	"errors"
	"os"

	"storage_to_git/models"
)

func LoadOrInitVersions(filePath string, project models.Project) (models.VersionMap, error) {
	_, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		versions := make(models.VersionMap)

		if project.Storage != nil {
			versions["cf"] = 0
		}

		for _, ext := range project.Extensions {
			versions[ext.ExtensionName] = 0
		}

		err = SaveVersions(filePath, versions)
		if err != nil {
			return nil, err
		}
		return versions, nil
	}

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

func SaveVersions(filePath string, versions models.VersionMap) error {
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

