package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath resolves a file path to an absolute path
// Handles:
// - Relative paths (resolved relative to the config file's directory)
// - Tilde expansion (~/path)
// - Absolute paths (returned as-is)
// - Empty strings (returned as-is)
func ResolvePath(path string, baseDir string) (string, error) {
	if path == "" {
		return path, nil
	}

	// If it's already absolute, return as-is
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	// Handle tilde expansion
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
		return filepath.Clean(path), nil
	}

	// For relative paths, resolve relative to baseDir
	if baseDir == "" {
		// If no baseDir provided, use current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		baseDir = cwd
	}

	// Resolve relative to baseDir
	resolved := filepath.Join(baseDir, path)
	return filepath.Abs(resolved)
}

// IsFilePath checks if a string looks like a file path (not just a command name)
// Returns true if it contains path separators or starts with ./
func IsFilePath(path string) bool {
	if path == "" {
		return false
	}
	// Check if it contains path separators
	if strings.Contains(path, "/") || strings.Contains(path, "\\") {
		return true
	}
	// Check if it starts with ./
	if strings.HasPrefix(path, "./") {
		return true
	}
	// Check if it starts with ~/
	if strings.HasPrefix(path, "~/") {
		return true
	}
	return false
}

