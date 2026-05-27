package vfs

import (
	"context"
	"time"
)

// VFS defines workspace-relative file operations for vscode-lite.
type VFS interface {
	Root() string
	List(ctx context.Context, path string, opts ListOptions) (*ListResult, error)
	Read(ctx context.Context, path string) (*ReadResult, error)
	Write(ctx context.Context, path string, content []byte) error
	Stat(ctx context.Context, path string) (*FileInfo, error)
	Rename(ctx context.Context, oldPath, newPath string) error
	Delete(ctx context.Context, path string) error
}

type ListOptions struct {
	Hidden bool `json:"hidden"`
}

type ListResult struct {
	Path    string     `json:"path"`
	Entries []FileInfo `json:"entries"`
}

type ReadResult struct {
	Path     string    `json:"path"`
	Content  string    `json:"content"`
	ModTime  time.Time `json:"modTime"`
	Size     int64     `json:"size"`
	Language string    `json:"language,omitempty"`
}

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}
