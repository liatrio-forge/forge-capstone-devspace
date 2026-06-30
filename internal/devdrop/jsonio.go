package devdrop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func readJSON(path string, into any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}

func writeJSON(path string, value any, perm os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func missing(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
