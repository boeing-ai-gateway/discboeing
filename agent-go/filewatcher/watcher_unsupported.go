//go:build !linux

package filewatcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Kind identifies the type of change detected for a path.
type Kind string

const (
	// Created means a file-system entry now exists but was absent from the
	// previous snapshot.
	Created Kind = "created"
	// Modified means a file-system entry exists in both snapshots but its type,
	// size, permissions, or modification time changed.
	Modified Kind = "modified"
	// Deleted means a file-system entry existed in the previous snapshot but is
	// now absent.
	Deleted Kind = "deleted"
)

// Entry describes a file-system entry relative to the watched root.
type Entry struct {
	Path    string      `json:"path"`
	IsDir   bool        `json:"isDir"`
	Size    int64       `json:"size"`
	Mode    fs.FileMode `json:"mode"`
	ModTime time.Time   `json:"modTime"`
}

// Change describes the authoritative result of a tree diff.
type Change struct {
	Kind  Kind   `json:"kind"`
	Path  string `json:"path"`
	Entry *Entry `json:"entry,omitempty"`
}

// Batch contains all changes detected by one coalesced scan.
type Batch struct {
	Root     string   `json:"root"`
	Changes  []Change `json:"changes"`
	Resync   bool     `json:"resync,omitempty"`
	Snapshot []Entry  `json:"snapshot,omitempty"`
}

// Options configures a Watcher.
type Options struct {
	// Debounce controls how long the watcher waits after the first kernel event
	// before rescanning. Zero uses a conservative default.
	Debounce time.Duration
	// Buffer controls the Events and Errors channel buffers. Zero uses a default.
	Buffer int
	// IncludeInitial emits every existing path as Created after construction.
	IncludeInitial bool
	// ResyncInterval controls periodic full-tree scans. Zero uses a conservative
	// default; a negative value disables periodic resyncs.
	ResyncInterval time.Duration
	// RespectGitignore prunes paths ignored by .gitignore files under the
	// watched root. It is enabled by default.
	RespectGitignore *bool
}

// Watcher is unavailable on non-Linux platforms.
type Watcher struct {
	root   string
	events chan Batch
	errors chan error
}

// New reports that workspace file watching is unavailable on non-Linux
// platforms. The production agent runs in Linux sandboxes, but this stub keeps
// packages that reference filewatcher buildable on developer and CI hosts.
func New(root string, opts Options) (*Watcher, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("watch root %q is not a directory", cleanRoot)
	}

	return nil, fmt.Errorf("file watcher is unsupported on %s", runtime.GOOS)
}

// Events returns a closed event channel.
func (w *Watcher) Events() <-chan Batch {
	if w == nil || w.events == nil {
		ch := make(chan Batch)
		close(ch)
		return ch
	}
	return w.events
}

// Errors returns a closed error channel.
func (w *Watcher) Errors() <-chan error {
	if w == nil || w.errors == nil {
		ch := make(chan error)
		close(ch)
		return ch
	}
	return w.errors
}

// Root returns the watched root path.
func (w *Watcher) Root() string {
	if w == nil {
		return ""
	}
	return w.root
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	return nil
}
