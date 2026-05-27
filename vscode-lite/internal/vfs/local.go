package vfs

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LocalVFS struct {
	root string
}

func NewLocal(root string) (*LocalVFS, error) {
	resolvedRoot, err := resolveWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	return &LocalVFS{root: resolvedRoot}, nil
}

func (v *LocalVFS) Root() string {
	return v.root
}

func (v *LocalVFS) List(_ context.Context, path string, opts ListOptions) (*ListResult, error) {
	absPath, err := resolveExisting(v.root, path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	infos := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if !opts.Hidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		entryPath := filepath.Join(absPath, entry.Name())
		if target, err := filepath.EvalSymlinks(entryPath); err == nil && !isWithinRoot(v.root, target) {
			continue
		}
		rel, err := filepath.Rel(v.root, entryPath)
		if err != nil {
			continue
		}
		infos = append(infos, FileInfo{
			Name:    entry.Name(),
			Path:    toSlashPath(rel),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].IsDir != infos[j].IsDir {
			return infos[i].IsDir
		}
		return strings.ToLower(infos[i].Name) < strings.ToLower(infos[j].Name)
	})
	clean, err := cleanRelativePath(path)
	if err != nil {
		return nil, err
	}
	return &ListResult{Path: toSlashPath(clean), Entries: infos}, nil
}

func (v *LocalVFS) Read(_ context.Context, path string) (*ReadResult, error) {
	absPath, err := resolveExisting(v.root, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	clean, err := cleanRelativePath(path)
	if err != nil {
		return nil, err
	}
	return &ReadResult{
		Path:    toSlashPath(clean),
		Content: string(content),
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}, nil
}

func (v *LocalVFS) Write(_ context.Context, path string, content []byte) error {
	absPath, err := resolveForWrite(v.root, path)
	if err != nil {
		return err
	}
	return os.WriteFile(absPath, content, 0o644)
}

func (v *LocalVFS) Stat(_ context.Context, path string) (*FileInfo, error) {
	absPath, err := resolveExisting(v.root, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(v.root, absPath)
	if err != nil {
		return nil, err
	}
	return &FileInfo{Name: info.Name(), Path: toSlashPath(rel), IsDir: info.IsDir(), Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (v *LocalVFS) Rename(_ context.Context, oldPath, newPath string) error {
	oldAbs, err := resolveExisting(v.root, oldPath)
	if err != nil {
		return err
	}
	newAbs, err := resolveForWrite(v.root, newPath)
	if err != nil {
		return err
	}
	return os.Rename(oldAbs, newAbs)
}

func (v *LocalVFS) Delete(_ context.Context, path string) error {
	absPath, err := resolveExisting(v.root, path)
	if err != nil {
		return err
	}
	if absPath == v.root {
		return ErrInvalidPath
	}
	return os.RemoveAll(absPath)
}
