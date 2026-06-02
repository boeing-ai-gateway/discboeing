package filewatcher

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWatcherDetectsCreateModifyDelete(t *testing.T) {
	root := t.TempDir()
	watcher := newTestWatcher(t, root)

	file := filepath.Join(root, "note.txt")
	if err := os.WriteFile(file, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, watcher, Created, "note.txt")

	if err := os.WriteFile(file, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, watcher, Modified, "note.txt")

	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, watcher, Deleted, "note.txt")
}

func TestWatcherDetectsNewNestedDirectories(t *testing.T) {
	root := t.TempDir()
	watcher := newTestWatcher(t, root)

	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForChange(t, watcher, Created, "a/b/file.txt")

	if err := os.WriteFile(filepath.Join(nested, "later.txt"), []byte("later"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, watcher, Created, "a/b/later.txt")
}

func TestWatcherIncludeInitial(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	watcher, err := New(root, Options{Debounce: time.Millisecond, IncludeInitial: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := watcher.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	batch := waitForBatch(t, watcher)
	assertBatchHasChange(t, batch, Created, "dir")
	assertBatchHasChange(t, batch, Created, "dir/file.txt")
}

func TestWatcherCloseClosesChannels(t *testing.T) {
	watcher := newTestWatcher(t, t.TempDir())
	if err := watcher.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case _, ok := <-watcher.Events():
		if ok {
			t.Fatal("Events() channel is still open")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Events() channel to close")
	}

	if err := watcher.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestWatcherDetectsSameSizeRewrite(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "same-size.txt")
	if err := os.WriteFile(file, []byte("aaaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher := newTestWatcher(t, root)

	time.Sleep(2 * time.Millisecond)
	if err := os.WriteFile(file, []byte("bbbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, watcher, Modified, "same-size.txt")
}

func TestWatcherPrunesGitignoredDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.js"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher := newTestWatcher(t, root)

	if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.js"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertNoChangeForPath(t, watcher, "node_modules/pkg/index.js", 150*time.Millisecond)

	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("visible"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, watcher, Created, "keep.txt")

	if err := os.WriteFile(filepath.Join(root, "ignored.log"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertNoChangeForPath(t, watcher, "ignored.log", 150*time.Millisecond)
}

func TestWatcherLoadsNestedGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", ".gitignore"), []byte("generated/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	watcher := newTestWatcher(t, root)
	if err := os.WriteFile(filepath.Join(root, "src", "generated", "client.go"), []byte("package generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertNoChangeForPath(t, watcher, "src/generated/client.go", 150*time.Millisecond)

	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForChange(t, watcher, Created, "src/main.go")
}

func TestWatcherCanDisableGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored", "file.txt"), []byte("visible when disabled"), 0o644); err != nil {
		t.Fatal(err)
	}

	watcher, err := New(root, Options{
		Debounce:         time.Millisecond,
		IncludeInitial:   true,
		RespectGitignore: new(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := watcher.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	batch := waitForBatch(t, watcher)
	assertBatchHasChange(t, batch, Created, "ignored")
	assertBatchHasChange(t, batch, Created, "ignored/file.txt")
}

func TestWatcherEmitsPeriodicResyncSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, Options{
		Debounce:       time.Millisecond,
		ResyncInterval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := watcher.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	batch := waitForResyncBatch(t, watcher)
	if !batch.Resync {
		t.Fatalf("Resync = false for %#v", batch)
	}
	assertSnapshotHasPath(t, batch, "file.txt")
}

func TestWatcherRandomizedTreeConverges(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		t.Run(strconv.FormatUint(seed, 10), func(t *testing.T) {
			root := t.TempDir()
			watcher := newTestWatcher(t, root)
			observed := map[string]entrySignature{}
			rng := rand.New(rand.NewPCG(seed, seed^0xa5a5a5a5a5a5a5a5))

			for round := range 40 {
				ops := rng.IntN(8) + 1
				for op := range ops {
					if err := applyRandomOperation(root, rng, round, op); err != nil {
						t.Fatalf("seed %d round %d op %d: %v", seed, round, op, err)
					}
				}

				expected, err := snapshotSignatures(root)
				if err != nil {
					t.Fatalf("seed %d round %d snapshot: %v", seed, round, err)
				}
				waitForObservedSignatures(t, watcher, observed, expected)
			}
		})
	}
}

func TestWatcherRandomizedAtomicReplaceBursts(t *testing.T) {
	for seed := uint64(100); seed < 110; seed++ {
		t.Run(strconv.FormatUint(seed, 10), func(t *testing.T) {
			root := t.TempDir()
			watcher := newTestWatcher(t, root)
			observed := map[string]entrySignature{}
			rng := rand.New(rand.NewPCG(seed, seed^0x5a5a5a5a5a5a5a5a))

			for round := range 25 {
				dir := filepath.Join(root, fmt.Sprintf("dir-%02d", rng.IntN(5)))
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				for i := range 12 {
					name := filepath.Join(dir, fmt.Sprintf("file-%02d.txt", rng.IntN(8)))
					if err := atomicReplace(name, randomContent(rng)); err != nil {
						t.Fatalf("seed %d round %d replace %d: %v", seed, round, i, err)
					}
				}
				for i := range 4 {
					name := filepath.Join(dir, fmt.Sprintf("file-%02d.txt", rng.IntN(8)))
					if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
						t.Fatalf("seed %d round %d remove %d: %v", seed, round, i, err)
					}
				}

				expected, err := snapshotSignatures(root)
				if err != nil {
					t.Fatalf("seed %d round %d snapshot: %v", seed, round, err)
				}
				waitForObservedSignatures(t, watcher, observed, expected)
			}
		})
	}
}

func newTestWatcher(t *testing.T, root string) *Watcher {
	t.Helper()
	watcher, err := New(root, Options{Debounce: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := watcher.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return watcher
}

func waitForChange(t *testing.T, watcher *Watcher, kind Kind, path string) Change {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case batch, ok := <-watcher.Events():
			if !ok {
				t.Fatalf("Events() closed before %s %s", kind, path)
			}
			for _, change := range batch.Changes {
				if change.Kind == kind && change.Path == path {
					return change
				}
			}
		case err, ok := <-watcher.Errors():
			if ok {
				t.Fatalf("watcher error: %v", err)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s %s", kind, path)
		}
	}
}

func waitForBatch(t *testing.T, watcher *Watcher) Batch {
	t.Helper()
	select {
	case batch, ok := <-watcher.Events():
		if !ok {
			t.Fatal("Events() closed before batch")
		}
		return batch
	case err, ok := <-watcher.Errors():
		if ok {
			t.Fatalf("watcher error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for batch")
	}
	return Batch{}
}

func assertBatchHasChange(t *testing.T, batch Batch, kind Kind, path string) {
	t.Helper()
	for _, change := range batch.Changes {
		if change.Kind == kind && change.Path == path {
			return
		}
	}
	t.Fatalf("batch missing %s %s: %#v", kind, path, batch.Changes)
}

func assertNoChangeForPath(t *testing.T, watcher *Watcher, path string, duration time.Duration) {
	t.Helper()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case batch, ok := <-watcher.Events():
			if !ok {
				t.Fatal("Events() closed")
			}
			for _, change := range batch.Changes {
				if change.Path == path {
					t.Fatalf("unexpected change for %s: %#v", path, change)
				}
			}
			for _, entry := range batch.Snapshot {
				if entry.Path == path {
					t.Fatalf("unexpected snapshot entry for %s: %#v", path, entry)
				}
			}
		case err, ok := <-watcher.Errors():
			if ok {
				t.Fatalf("watcher error: %v", err)
			}
		case <-timer.C:
			return
		}
	}
}

func waitForResyncBatch(t *testing.T, watcher *Watcher) Batch {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case batch, ok := <-watcher.Events():
			if !ok {
				t.Fatal("Events() closed before resync")
			}
			if batch.Resync {
				return batch
			}
		case err, ok := <-watcher.Errors():
			if ok {
				t.Fatalf("watcher error: %v", err)
			}
		case <-deadline:
			t.Fatal("timed out waiting for resync")
		}
	}
}

func assertSnapshotHasPath(t *testing.T, batch Batch, path string) {
	t.Helper()
	for _, entry := range batch.Snapshot {
		if entry.Path == path {
			return
		}
	}
	t.Fatalf("snapshot missing %s: %#v", path, batch.Snapshot)
}

type entrySignature struct {
	isDir bool
	size  int64
	mode  os.FileMode
}

func waitForObservedSignatures(
	t *testing.T,
	watcher *Watcher,
	observed, expected map[string]entrySignature,
) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if equalSignatures(observed, expected) {
			return
		}
		select {
		case batch, ok := <-watcher.Events():
			if !ok {
				t.Fatalf("Events() closed before convergence\nobserved: %s\nexpected: %s", formatSignatures(observed), formatSignatures(expected))
			}
			applyBatchSignatures(observed, batch)
		case err, ok := <-watcher.Errors():
			if ok {
				t.Fatalf("watcher error: %v", err)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for convergence\nobserved: %s\nexpected: %s", formatSignatures(observed), formatSignatures(expected))
		}
	}
}

func applyBatchSignatures(observed map[string]entrySignature, batch Batch) {
	for _, change := range batch.Changes {
		switch change.Kind {
		case Created, Modified:
			if change.Entry != nil {
				observed[change.Path] = entrySignature{
					isDir: change.Entry.IsDir,
					size:  change.Entry.Size,
					mode:  change.Entry.Mode.Perm(),
				}
			}
		case Deleted:
			delete(observed, change.Path)
		}
	}
}

func snapshotSignatures(root string) (map[string]entrySignature, error) {
	entries := map[string]entrySignature{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries[filepath.ToSlash(rel)] = entrySignature{
			isDir: info.IsDir(),
			size:  info.Size(),
			mode:  info.Mode().Perm(),
		}
		return nil
	})
	return entries, err
}

func equalSignatures(a, b map[string]entrySignature) bool {
	if len(a) != len(b) {
		return false
	}
	for path, aEntry := range a {
		if bEntry, ok := b[path]; !ok || aEntry != bEntry {
			return false
		}
	}
	return true
}

func formatSignatures(entries map[string]entrySignature) string {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var out strings.Builder
	for _, path := range paths {
		entry := entries[path]
		if out.Len() > 0 {
			out.WriteString(", ")
		}
		kind := "file"
		if entry.isDir {
			kind = "dir"
		}
		out.WriteString(fmt.Sprintf("%s:%s:%d:%o", path, kind, entry.size, entry.mode))
	}
	return out.String()
}

func applyRandomOperation(root string, rng *rand.Rand, round, op int) error {
	files, dirs, err := listTree(root)
	if err != nil {
		return err
	}
	switch rng.IntN(9) {
	case 0:
		parent := pickDir(dirs, rng)
		return os.MkdirAll(filepath.Join(root, filepath.FromSlash(parent), randomName(rng, "dir", round, op)), 0o755)
	case 1:
		parent := pickDir(dirs, rng)
		return os.WriteFile(
			filepath.Join(root, filepath.FromSlash(parent), randomName(rng, "file", round, op)+".txt"),
			[]byte(randomContent(rng)),
			0o644,
		)
	case 2:
		if len(files) == 0 {
			return nil
		}
		return os.WriteFile(filepath.Join(root, filepath.FromSlash(files[rng.IntN(len(files))])), []byte(randomContent(rng)), 0o644)
	case 3:
		if len(files) == 0 {
			return nil
		}
		file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(files[rng.IntN(len(files))])), os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return err
		}
		if _, err := file.WriteString(randomContent(rng)); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	case 4:
		if len(files) == 0 {
			return nil
		}
		return os.Remove(filepath.Join(root, filepath.FromSlash(files[rng.IntN(len(files))])))
	case 5:
		removable := nonRootDirs(dirs)
		if len(removable) == 0 {
			return nil
		}
		return os.RemoveAll(filepath.Join(root, filepath.FromSlash(removable[rng.IntN(len(removable))])))
	case 6:
		if len(files) == 0 {
			return nil
		}
		oldPath := filepath.Join(root, filepath.FromSlash(files[rng.IntN(len(files))]))
		parent := pickDir(dirs, rng)
		newPath := filepath.Join(root, filepath.FromSlash(parent), randomName(rng, "renamed", round, op)+".txt")
		return os.Rename(oldPath, newPath)
	case 7:
		if len(files) == 0 {
			return nil
		}
		mode := os.FileMode(0o600)
		if rng.IntN(2) == 0 {
			mode = 0o644
		}
		return os.Chmod(filepath.Join(root, filepath.FromSlash(files[rng.IntN(len(files))])), mode)
	case 8:
		parent := pickDir(dirs, rng)
		return atomicReplace(filepath.Join(root, filepath.FromSlash(parent), randomName(rng, "atomic", round, op)+".txt"), randomContent(rng))
	default:
		return nil
	}
}

func listTree(root string) ([]string, []string, error) {
	files := []string{}
	dirs := []string{"."}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			dirs = append(dirs, rel)
		} else {
			files = append(files, rel)
		}
		return nil
	})
	return files, dirs, err
}

func pickDir(dirs []string, rng *rand.Rand) string {
	if len(dirs) == 0 {
		return "."
	}
	return dirs[rng.IntN(len(dirs))]
}

func nonRootDirs(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir != "." {
			out = append(out, dir)
		}
	}
	return out
}

func randomName(rng *rand.Rand, prefix string, round, op int) string {
	return fmt.Sprintf("%s-%02d-%02d-%04d", prefix, round, op, rng.IntN(10000))
}

func randomContent(rng *rand.Rand) string {
	size := rng.IntN(256) + 1
	var out strings.Builder
	for range size {
		out.WriteByte(byte('a' + rng.IntN(26)))
	}
	return out.String()
}

func atomicReplace(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
