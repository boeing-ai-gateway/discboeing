package vfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrInvalidPath = errors.New("invalid workspace path")

func cleanRelativePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return ".", nil
	}
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "/") || filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: absolute paths are not allowed", ErrInvalidPath)
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("%w: traversal is not allowed", ErrInvalidPath)
		}
	}
	return strings.Join(parts, string(filepath.Separator)), nil
}

func resolveWorkspaceRoot(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace is not a directory: %s", absRoot)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", err
	}
	return realRoot, nil
}

func resolveExisting(root, rel string) (string, error) {
	clean, err := cleanRelativePath(rel)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, clean)
	realPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if !isWithinRoot(root, realPath) {
		return "", fmt.Errorf("%w: path escapes workspace", ErrInvalidPath)
	}
	return realPath, nil
}

func resolveForWrite(root, rel string) (string, error) {
	clean, err := cleanRelativePath(rel)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, clean)
	parent := filepath.Dir(candidate)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !isWithinRoot(root, realParent) {
		return "", fmt.Errorf("%w: parent escapes workspace", ErrInvalidPath)
	}
	return filepath.Join(realParent, filepath.Base(candidate)), nil
}

func isWithinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func toSlashPath(path string) string {
	path = filepath.ToSlash(path)
	if path == "." {
		return "."
	}
	return strings.TrimPrefix(path, "./")
}
