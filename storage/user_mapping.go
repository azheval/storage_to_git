package storage

import (
	"encoding/csv"
	"io"
	"os"
	"storage_to_git/models"
)

// LoadUserMappings reads user mapping data from a CSV file.
func LoadUserMappings(filePath string) ([]models.UserMapping, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'

	var mappings []models.UserMapping

	// Skip header row
	_, err = reader.Read()
	if err != nil {
		if err == io.EOF {
			return mappings, nil // Empty file is valid
		}
		return nil, err
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 3 {
			continue
		}

		mapping := models.UserMapping{
			StorageUser:   record[0],
			GitUser:  record[1],
			GitEmail: record[2],
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}
