// Package filewatcher watches a directory tree and reports stable file-system
// changes.
//
// The implementation is Linux-only. It uses inotify as a wake-up signal and
// diffs full directory snapshots after a short debounce window. That makes the
// public stream resilient to editor atomic writes, rename bursts, recursive
// directory races, and inotify queue overflows: consumers receive the resulting
// tree changes, not lossy raw kernel events.
package filewatcher

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	defaultDebounce = 75 * time.Millisecond
	defaultBuffer   = 32
	defaultResync   = 30 * time.Second
	readBufferSize  = 64 * 1024
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

// Watcher watches one directory tree.
type Watcher struct {
	root     string
	fd       int
	events   chan Batch
	errors   chan error
	done     chan struct{}
	closed   chan struct{}
	closeMu  sync.Mutex
	closeErr error

	mu       sync.Mutex
	byPath   map[string]int
	snap     map[string]Entry
	debounce time.Duration
	resync   time.Duration
	ignore   bool
	readBuf  []byte
}

// New starts watching root. Root must be an existing directory.
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

	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return nil, fmt.Errorf("create inotify instance: %w", err)
	}

	buffer := opts.Buffer
	if buffer <= 0 {
		buffer = defaultBuffer
	}
	debounce := opts.Debounce
	if debounce <= 0 {
		debounce = defaultDebounce
	}
	resync := opts.ResyncInterval
	if resync == 0 {
		resync = defaultResync
	}
	ignore := true
	if opts.RespectGitignore != nil {
		ignore = *opts.RespectGitignore
	}

	w := &Watcher{
		root:     cleanRoot,
		fd:       fd,
		events:   make(chan Batch, buffer),
		errors:   make(chan error, buffer),
		done:     make(chan struct{}),
		closed:   make(chan struct{}),
		byPath:   make(map[string]int),
		debounce: debounce,
		resync:   resync,
		ignore:   ignore,
		readBuf:  make([]byte, readBufferSize),
	}

	snap, err := w.scanLocked()
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	w.snap = snap

	go w.run()

	if opts.IncludeInitial {
		changes := make([]Change, 0, len(snap))
		paths := sortedKeys(snap)
		for _, path := range paths {
			entry := snap[path]
			changes = append(changes, Change{Kind: Created, Path: path, Entry: &entry})
		}
		if len(changes) > 0 {
			w.events <- Batch{Root: w.root, Changes: changes}
		}
	}

	return w, nil
}

// Events returns batches of coalesced file-system changes. The channel closes
// after Close completes or the watcher stops because of an unrecoverable error.
func (w *Watcher) Events() <-chan Batch {
	return w.events
}

// Errors returns recoverable watch and scan errors. The channel closes with
// Events.
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// Root returns the absolute watched root path.
func (w *Watcher) Root() string {
	return w.root
}

// Close stops the watcher. It is safe to call multiple times.
func (w *Watcher) Close() error {
	w.closeMu.Lock()
	select {
	case <-w.done:
		w.closeMu.Unlock()
		<-w.closed
		return w.closeErr
	default:
		close(w.done)
		w.closeErr = unix.Close(w.fd)
	}
	w.closeMu.Unlock()

	<-w.closed
	return w.closeErr
}

func (w *Watcher) run() {
	defer close(w.closed)
	defer close(w.events)
	defer close(w.errors)

	var timer *time.Timer
	var timerC <-chan time.Time
	poll := time.NewTicker(10 * time.Millisecond)
	var resyncC <-chan time.Time
	var resyncTimer *time.Timer
	if w.resync > 0 {
		resyncTimer = time.NewTimer(w.resync)
		resyncC = resyncTimer.C
	}
	defer poll.Stop()
	defer func() {
		if timer != nil {
			timer.Stop()
		}
		if resyncTimer != nil {
			resyncTimer.Stop()
		}
	}()

	for {
		select {
		case <-w.done:
			return
		case <-timerC:
			timerC = nil
			w.rescanAndPublish(false)
		case <-resyncC:
			w.rescanAndPublish(true)
			resyncTimer.Reset(w.resync)
			resyncC = resyncTimer.C
		case <-poll.C:
			hadEvent, overflow, err := w.readKernelEvents()
			if err != nil {
				if w.isClosed() || errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINVAL) {
					return
				}
				w.reportError(fmt.Errorf("read inotify events: %w", err))
				return
			}
			if overflow {
				w.rescanAndPublish(true)
			}
			if !hadEvent {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(w.debounce)
			} else {
				if !timer.Stop() && timerC != nil {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(w.debounce)
			}
			timerC = timer.C
		}
	}
}

func (w *Watcher) readKernelEvents() (bool, bool, error) {
	n, err := unix.Read(w.fd, w.readBuf)
	if err != nil {
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
			return false, false, nil
		}
		return false, false, err
	}
	if n == 0 {
		return false, false, nil
	}

	overflow := false
	for offset := 0; offset+unix.SizeofInotifyEvent <= n; {
		raw := (*unix.InotifyEvent)(unsafe.Pointer(&w.readBuf[offset]))
		if raw.Mask&unix.IN_Q_OVERFLOW != 0 {
			w.reportError(errors.New("inotify queue overflow; rescanning tree"))
			overflow = true
		}
		offset += unix.SizeofInotifyEvent + int(raw.Len)
	}
	return true, overflow, nil
}

func (w *Watcher) rescanAndPublish(resync bool) {
	w.mu.Lock()
	next, err := w.scanLocked()
	if err != nil {
		w.mu.Unlock()
		w.reportError(err)
		return
	}
	changes := diffSnapshots(w.snap, next)
	w.snap = next
	w.mu.Unlock()

	if len(changes) == 0 && !resync {
		return
	}

	batch := Batch{Root: w.root, Changes: changes, Resync: resync}
	if resync {
		batch.Snapshot = snapshotEntries(next)
	}
	select {
	case w.events <- batch:
	case <-w.done:
	}
}

func (w *Watcher) scanLocked() (map[string]Entry, error) {
	next := make(map[string]Entry)
	dirs := make(map[string]struct{})
	ignores := newIgnoreStack(w.root, w.ignore)

	err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == w.root {
			dirs["."] = struct{}{}
			ignores.load(path)
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.Name() == ".git" && d.IsDir() {
			return filepath.SkipDir
		}

		info, err := d.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}

		rel, err := filepath.Rel(w.root, path)
		if err != nil {
			return err
		}
		rel = normalizePath(rel)
		if ignores.ignored(rel, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			ignores.load(path)
		}
		next[rel] = Entry{
			Path:    rel,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
		}
		if info.IsDir() {
			dirs[rel] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan watched tree: %w", err)
	}

	w.syncWatchesLocked(dirs)
	return next, nil
}

func (w *Watcher) syncWatchesLocked(dirs map[string]struct{}) {
	for rel := range dirs {
		if _, ok := w.byPath[rel]; ok {
			continue
		}
		wd, err := unix.InotifyAddWatch(w.fd, w.abs(rel), watchMask)
		if err != nil {
			w.reportError(fmt.Errorf("watch directory %q: %w", rel, err))
			continue
		}
		w.byPath[rel] = wd
	}

	for rel, wd := range w.byPath {
		if _, ok := dirs[rel]; ok {
			continue
		}
		_, _ = unix.InotifyRmWatch(w.fd, uint32(wd))
		delete(w.byPath, rel)
	}
}

const watchMask = unix.IN_CREATE | unix.IN_DELETE | unix.IN_DELETE_SELF | unix.IN_MODIFY |
	unix.IN_MOVED_FROM | unix.IN_MOVED_TO | unix.IN_MOVE_SELF | unix.IN_ATTRIB |
	unix.IN_CLOSE_WRITE | unix.IN_ONLYDIR | unix.IN_DONT_FOLLOW | unix.IN_EXCL_UNLINK

func diffSnapshots(prev, next map[string]Entry) []Change {
	paths := make(map[string]struct{}, len(prev)+len(next))
	for path := range prev {
		paths[path] = struct{}{}
	}
	for path := range next {
		paths[path] = struct{}{}
	}

	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	changes := make([]Change, 0)
	for _, path := range ordered {
		oldEntry, hadOld := prev[path]
		newEntry, hasNew := next[path]
		switch {
		case !hadOld && hasNew:
			entry := newEntry
			changes = append(changes, Change{Kind: Created, Path: path, Entry: &entry})
		case hadOld && !hasNew:
			changes = append(changes, Change{Kind: Deleted, Path: path})
		case hadOld && hasNew && entryChanged(oldEntry, newEntry):
			entry := newEntry
			changes = append(changes, Change{Kind: Modified, Path: path, Entry: &entry})
		}
	}
	return changes
}

func entryChanged(a, b Entry) bool {
	return a.IsDir != b.IsDir || a.Size != b.Size || a.Mode != b.Mode || !a.ModTime.Equal(b.ModTime)
}

func sortedKeys(entries map[string]Entry) []string {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func snapshotEntries(entries map[string]Entry) []Entry {
	paths := sortedKeys(entries)
	out := make([]Entry, 0, len(paths))
	for _, path := range paths {
		out = append(out, entries[path])
	}
	return out
}

func (w *Watcher) abs(rel string) string {
	if rel == "." {
		return w.root
	}
	return filepath.Join(w.root, filepath.FromSlash(rel))
}

func normalizePath(path string) string {
	if path == "" || path == "." {
		return "."
	}
	return filepath.ToSlash(strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator)))
}

func (w *Watcher) reportError(err error) {
	select {
	case w.errors <- err:
	case <-w.done:
	default:
	}
}

func (w *Watcher) isClosed() bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

type ignoreStack struct {
	root     string
	enabled  bool
	patterns []ignorePattern
}

type ignorePattern struct {
	re      *regexp.Regexp
	base    string
	negated bool
	dirOnly bool
}

func newIgnoreStack(root string, enabled bool) *ignoreStack {
	return &ignoreStack{root: root, enabled: enabled}
}

func (s *ignoreStack) load(dir string) {
	if !s.enabled {
		return
	}
	file, err := os.Open(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return
	}
	defer file.Close()

	base, err := filepath.Rel(s.root, dir)
	if err != nil {
		return
	}
	base = normalizePath(base)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if pattern, ok := parseIgnorePattern(scanner.Text(), base); ok {
			s.patterns = append(s.patterns, pattern)
		}
	}
}

func (s *ignoreStack) ignored(path string, isDir bool) bool {
	if !s.enabled {
		return false
	}
	ignored := false
	for _, pattern := range s.patterns {
		if pattern.dirOnly && !isDir {
			continue
		}
		matchPath, ok := pathRelativeToBase(path, pattern.base)
		if !ok {
			continue
		}
		if pattern.re.MatchString(matchPath) {
			ignored = !pattern.negated
		}
	}
	return ignored
}

func pathRelativeToBase(path, base string) (string, bool) {
	if base == "." {
		return path, true
	}
	if path == base {
		return ".", true
	}
	prefix := base + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	return strings.TrimPrefix(path, prefix), true
}

func parseIgnorePattern(line, base string) (ignorePattern, bool) {
	line = strings.TrimRight(line, " \t\r")
	if line == "" || line[0] == '#' {
		return ignorePattern{}, false
	}

	pattern := ignorePattern{base: base}
	escaped := false
	if len(line) >= 2 && line[0] == '\\' && (line[1] == '!' || line[1] == '#') {
		line = line[1:]
		escaped = true
	}
	if !escaped && line[0] == '!' {
		pattern.negated = true
		line = line[1:]
	}
	if line == "" {
		return ignorePattern{}, false
	}

	if strings.HasSuffix(line, "/") {
		pattern.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	line = strings.TrimPrefix(line, "/")
	if line == "" {
		return ignorePattern{}, false
	}

	re, err := regexp.Compile(ignorePatternRegexp(line))
	if err != nil {
		return ignorePattern{}, false
	}
	pattern.re = re
	return pattern, true
}

func ignorePatternRegexp(pattern string) string {
	var out strings.Builder
	out.WriteString("^")
	if !strings.Contains(pattern, "/") {
		out.WriteString("(?:.*/)?")
	}

	for i := 0; i < len(pattern); {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					out.WriteString("(?:.*/)?")
					i += 3
				} else {
					out.WriteString(".*")
					i += 2
				}
				continue
			}
			out.WriteString("[^/]*")
		case '?':
			out.WriteString("[^/]")
		case '[':
			end := i + 1
			for end < len(pattern) && pattern[end] != ']' {
				end++
			}
			if end < len(pattern) {
				out.WriteString(pattern[i : end+1])
				i = end
			} else {
				out.WriteString("\\[")
			}
		case '\\':
			if i+1 < len(pattern) {
				i++
				out.WriteString(regexp.QuoteMeta(string(pattern[i])))
			} else {
				out.WriteString("\\\\")
			}
		default:
			out.WriteString(regexp.QuoteMeta(string(ch)))
		}
		i++
	}
	out.WriteString("(?:/.*)?$")
	return out.String()
}
