package bar

import (
	"os"
	"path/filepath"
)

func homeDir() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".mkdev"), nil
}

func logsDir() (string, error) {
	h, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "logs"), nil
}
