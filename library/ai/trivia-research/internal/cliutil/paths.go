package cliutil

import (
	"os"
	"path/filepath"
)

func CacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "trivia-research-pp-cli")
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "trivia-research-pp-cli")
}

func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "trivia-research-pp-cli")
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
