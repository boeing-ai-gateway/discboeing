package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
)

// LocalProvider implements Provider using the local git CLI.
// Workspaces are cloned directly to {baseDir}/{projectID}/workspaces/{workspaceID}.
type LocalProvider struct {
	baseDir string

	workspaceSource WorkspaceSource

	projectMu    sync.Mutex
	projectLocks map[string]*sync.Mutex

	mu             sync.RWMutex
	workspaceIndex map[string]*workspaceInfo
}

// LocalProviderOption configures a LocalProvider.
type LocalProviderOption func(*LocalProvider)

// WithWorkspaceSource sets the workspace source for the provider.
// This enables EnsureWorkspaceByID and auto-recovery in GetWorkDir.
func WithWorkspaceSource(src WorkspaceSource) LocalProviderOption {
	return func(p *LocalProvider) {
		p.workspaceSource = src
	}
}

type workspaceInfo struct {
	projectID string
	workDir   string
	source    string
	isRemote  bool
}

// NewLocalProvider creates a new local git provider.
// baseDir is the root directory where workspaces will be stored.
// Structure: {baseDir}/{projectID}/workspaces/{workspaceID}/
func NewLocalProvider(baseDir string, opts ...LocalProviderOption) (*LocalProvider, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	p := &LocalProvider{
		baseDir:        baseDir,
		projectLocks:   make(map[string]*sync.Mutex),
		workspaceIndex: make(map[string]*workspaceInfo),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

func (p *LocalProvider) getProjectLock(projectID string) *sync.Mutex {
	p.projectMu.Lock()
	defer p.projectMu.Unlock()

	if lock, ok := p.projectLocks[projectID]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	p.projectLocks[projectID] = lock
	return lock
}

// EnsureWorkspace ensures a workspace has a working copy ready.
func (p *LocalProvider) EnsureWorkspace(ctx context.Context, projectID, workspaceID, source, ref string) (string, string, error) {
	p.mu.RLock()
	if info, ok := p.workspaceIndex[workspaceID]; ok {
		p.mu.RUnlock()
		commit, _ := p.currentCommit(ctx, info.workDir)
		return info.workDir, commit, nil
	}
	p.mu.RUnlock()

	projectLock := p.getProjectLock(projectID)
	projectLock.Lock()
	defer projectLock.Unlock()

	p.mu.RLock()
	if info, ok := p.workspaceIndex[workspaceID]; ok {
		p.mu.RUnlock()
		commit, _ := p.currentCommit(ctx, info.workDir)
		return info.workDir, commit, nil
	}
	p.mu.RUnlock()

	projectWorkspacesDir := filepath.Join(p.baseDir, projectID, "workspaces")
	if err := os.MkdirAll(projectWorkspacesDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create project workspaces directory: %w", err)
	}

	workDir := filepath.Join(projectWorkspacesDir, workspaceID)
	if p.isGitRepositoryPath(ctx, workDir) {
		info := &workspaceInfo{projectID: projectID, workDir: workDir, source: source, isRemote: IsGitURL(source)}
		p.mu.Lock()
		p.workspaceIndex[workspaceID] = info
		p.mu.Unlock()
		commit, _ := p.currentCommit(ctx, workDir)
		return workDir, commit, nil
	}

	cloneSource := source
	if !IsGitURL(source) {
		absSource, err := filepath.Abs(source)
		if err != nil {
			return "", "", fmt.Errorf("invalid path: %w", err)
		}
		if !p.isGitRepositoryPath(ctx, absSource) {
			return "", "", fmt.Errorf("%w: %s", ErrNotARepository, absSource)
		}
		cloneSource = absSource
	}

	_ = os.RemoveAll(workDir)
	if err := p.runGit(ctx, "", "-c", "core.autocrlf=false", "clone", "--", cloneSource, workDir); err != nil {
		_ = os.RemoveAll(workDir)
		return "", "", fmt.Errorf("%w: %w", ErrCloneFailed, err)
	}
	if err := p.runGit(ctx, workDir, "config", "core.autocrlf", "false"); err != nil {
		_ = os.RemoveAll(workDir)
		return "", "", fmt.Errorf("%w: %w", ErrCloneFailed, err)
	}

	if ref != "" {
		if err := p.checkoutRef(ctx, workDir, ref); err != nil {
			_ = os.RemoveAll(workDir)
			return "", "", fmt.Errorf("%w: %w", ErrCheckoutFailed, err)
		}
	}

	info := &workspaceInfo{projectID: projectID, workDir: workDir, source: cloneSource, isRemote: IsGitURL(source)}
	p.mu.Lock()
	p.workspaceIndex[workspaceID] = info
	p.mu.Unlock()

	commit, _ := p.currentCommit(ctx, workDir)
	return workDir, commit, nil
}

// EnsureWorkspaceByID ensures workspace is ready using only workspaceID.
func (p *LocalProvider) EnsureWorkspaceByID(ctx context.Context, workspaceID string) (string, string, error) {
	if p.workspaceSource == nil {
		return "", "", fmt.Errorf("workspace source not configured")
	}

	p.mu.RLock()
	if info, ok := p.workspaceIndex[workspaceID]; ok {
		p.mu.RUnlock()
		commit, _ := p.currentCommit(ctx, info.workDir)
		return info.workDir, commit, nil
	}
	p.mu.RUnlock()

	wsInfo, err := p.workspaceSource.GetWorkspaceInfo(ctx, workspaceID)
	if err != nil {
		return "", "", fmt.Errorf("workspace lookup failed: %w", err)
	}

	isRemote := IsGitURL(wsInfo.Path)
	if !isRemote && (wsInfo.SourceType == model.WorkspaceSourceTypeLocal || wsInfo.SourceType == model.WorkspaceSourceTypeManaged) {
		return p.registerLocalWorkspace(ctx, workspaceID, wsInfo.ProjectID, wsInfo.Path)
	}

	return p.EnsureWorkspace(ctx, wsInfo.ProjectID, workspaceID, wsInfo.Path, "")
}

func (p *LocalProvider) registerLocalWorkspace(ctx context.Context, workspaceID, projectID, localPath string) (string, string, error) {
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	commit, err := p.currentCommit(ctx, absPath)
	if err != nil {
		if errors.Is(err, ErrNotARepository) {
			return "", "", fmt.Errorf("%w: %s", ErrNotARepository, absPath)
		}
		return "", "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	p.mu.Lock()
	p.workspaceIndex[workspaceID] = &workspaceInfo{projectID: projectID, workDir: absPath, source: localPath, isRemote: false}
	p.mu.Unlock()

	return absPath, commit, nil
}

// Fetch fetches updates from remote.
func (p *LocalProvider) Fetch(ctx context.Context, workspaceID string) error {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	if err := p.runGit(ctx, workDir, "fetch", "--prune", "--tags", "origin"); err != nil {
		return fmt.Errorf("%w: %w", ErrFetchFailed, err)
	}
	return nil
}

// Checkout checks out a specific ref.
func (p *LocalProvider) Checkout(ctx context.Context, workspaceID, ref string) error {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	if err := p.checkoutRef(ctx, workDir, ref); err != nil {
		return fmt.Errorf("%w: %w", ErrCheckoutFailed, err)
	}
	return nil
}

// Status returns the current git status.
func (p *LocalProvider) Status(ctx context.Context, workspaceID string) (*Status, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	status := &Status{
		Staged:    []FileStatus{},
		Unstaged:  []FileStatus{},
		Untracked: []string{},
		IsClean:   true,
	}

	commit, err := p.currentCommit(ctx, workDir)
	if err != nil {
		return nil, err
	}
	status.Commit = commit
	if len(commit) >= 7 {
		status.CommitShort = commit[:7]
	}

	branch, err := p.currentBranch(ctx, workDir)
	if err == nil {
		status.Branch = branch
	}
	status.Ahead, status.Behind = p.countAheadBehind(ctx, workDir)

	output, err := p.runGitOutput(ctx, workDir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}

	entries := strings.Split(output, "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		if len(entry) < 3 {
			continue
		}

		xy := entry[:2]
		path := entry[3:]
		oldPath := ""
		if isRenameOrCopyStatus(xy) && i+1 < len(entries) {
			oldPath = entries[i+1]
			i++
		}

		if xy == "??" {
			status.IsClean = false
			status.Untracked = append(status.Untracked, path)
			continue
		}

		if isConflictStatus(xy) {
			status.HasConflicts = true
		}
		if xy[0] != ' ' || xy[1] != ' ' {
			status.IsClean = false
		}
		if xy[0] != ' ' && xy[0] != '?' {
			status.Staged = append(status.Staged, FileStatus{Path: path, Status: statusCodeToString(xy[0]), OldPath: oldPath})
		}
		if xy[1] != ' ' && xy[1] != '?' {
			status.Unstaged = append(status.Unstaged, FileStatus{Path: path, Status: statusCodeToString(xy[1]), OldPath: oldPath})
		}
	}

	return status, nil
}

// Diff returns file diffs.
func (p *LocalProvider) Diff(ctx context.Context, workspaceID string, opts DiffOptions) ([]FileDiff, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	contextLines := 3
	if opts.Context > 0 {
		contextLines = opts.Context
	}

	args := []string{"diff", "--find-renames", "--no-ext-diff", "--binary", fmt.Sprintf("-U%d", contextLines)}
	switch {
	case opts.BaseRef != "" && opts.HeadRef != "":
		args = append(args, opts.BaseRef, opts.HeadRef)
	case opts.BaseRef != "" && opts.Staged:
		args = append(args, "--cached", opts.BaseRef)
	case opts.BaseRef != "":
		args = append(args, opts.BaseRef)
	case opts.Staged:
		args = append(args, "--cached")
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		for _, path := range opts.Paths {
			args = append(args, filepath.ToSlash(path))
		}
	}

	output, err := p.runGitOutput(ctx, workDir, args...)
	if err != nil {
		if isInvalidRefError(err) {
			return nil, fmt.Errorf("%w", ErrInvalidRef)
		}
		return nil, err
	}
	return parseGitDiffOutput(output), nil
}

// Branches lists all branches.
func (p *LocalProvider) Branches(ctx context.Context, workspaceID string) ([]Branch, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	current, _ := p.currentBranch(ctx, workDir)
	var branches []Branch

	localOutput, err := p.runGitOutput(ctx, workDir, "for-each-ref", "--format=%(refname:short)%09%(objectname)%09%(upstream:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(strings.TrimRight(localOutput, "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 || fields[0] == "" {
			continue
		}
		branches = append(branches, Branch{
			Name:      fields[0],
			IsRemote:  false,
			IsCurrent: fields[0] == current,
			Commit:    fields[1],
			Upstream:  fields[2],
		})
	}

	remoteOutput, err := p.runGitOutput(ctx, workDir, "for-each-ref", "--format=%(refname:short)%09%(objectname)", "refs/remotes")
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(strings.TrimRight(remoteOutput, "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		name := fields[0]
		if name == "origin/HEAD" || strings.HasSuffix(name, "/HEAD") {
			continue
		}
		branches = append(branches, Branch{Name: name, IsRemote: true, Commit: fields[1]})
	}

	sort.Slice(branches, func(i, j int) bool { return branches[i].Name < branches[j].Name })
	return branches, nil
}

// FileTree returns the file listing at a specific ref.
func (p *LocalProvider) FileTree(ctx context.Context, workspaceID, ref string) ([]FileEntry, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	output, err := p.runGitOutput(ctx, workDir, "ls-tree", "-r", "-z", "--long", defaultRef(ref))
	if err != nil {
		if isInvalidRefError(err) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRef, defaultRef(ref))
		}
		return nil, err
	}

	entries := make([]FileEntry, 0)
	for record := range strings.SplitSeq(output, "\x00") {
		if record == "" {
			continue
		}
		meta, path, ok := strings.Cut(record, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) < 4 {
			continue
		}
		size, _ := strconv.ParseInt(fields[3], 10, 64)
		entries = append(entries, FileEntry{
			Path:  path,
			Name:  filepath.Base(path),
			IsDir: false,
			Size:  size,
			Mode:  fields[0],
		})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// ReadFile reads a file at a specific ref.
func (p *LocalProvider) ReadFile(ctx context.Context, workspaceID, ref, path string) ([]byte, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	if ref == "" {
		return os.ReadFile(filepath.Join(workDir, path))
	}

	gitPath := filepath.ToSlash(path)
	content, err := p.runGitBytes(ctx, workDir, "show", fmt.Sprintf("%s:%s", ref, gitPath))
	if err != nil {
		if isGitPathNotFoundError(err) {
			return nil, fmt.Errorf("%w: %s at %s", ErrNotFound, path, ref)
		}
		if isInvalidRefError(err) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRef, ref)
		}
		return nil, err
	}
	return content, nil
}

// WriteFile writes content to a file in the working tree.
func (p *LocalProvider) WriteFile(ctx context.Context, workspaceID, path string, content []byte) error {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	fullPath := filepath.Join(workDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0644)
}

// Stage stages files for commit.
func (p *LocalProvider) Stage(ctx context.Context, workspaceID string, paths []string) error {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	for _, path := range paths {
		if path == "." {
			if err := p.runGit(ctx, workDir, "add", "--all"); err != nil {
				return err
			}
			continue
		}
		if err := p.runGit(ctx, workDir, "add", "--", filepath.ToSlash(path)); err != nil {
			return err
		}
	}
	return nil
}

// Commit creates a commit with the staged changes.
func (p *LocalProvider) Commit(ctx context.Context, workspaceID, message, authorName, authorEmail string) (*Commit, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	args := []string{"commit", "-m", message}
	if authorName != "" && authorEmail != "" {
		args = append(args, "--author", fmt.Sprintf("%s <%s>", authorName, authorEmail))
	}

	env := map[string]string{}
	if committerName, committerEmail := p.loadCommitterIdentity(ctx, workDir); committerName != "" && committerEmail != "" {
		env["GIT_COMMITTER_NAME"] = committerName
		env["GIT_COMMITTER_EMAIL"] = committerEmail
	}

	if err := p.runGitWithEnv(ctx, workDir, env, args...); err != nil {
		return nil, err
	}
	return p.getCommit(ctx, workDir, "HEAD")
}

// Log returns commit history.
func (p *LocalProvider) Log(ctx context.Context, workspaceID string, opts LogOptions) ([]Commit, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	args := []string{
		"log",
		fmt.Sprintf("--max-count=%d", limit),
		fmt.Sprintf("--skip=%d", opts.Skip),
		"--date=iso-strict",
		"--format=%H%x1f%h%x1f%B%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%cI%x1f%P%x1e",
		defaultRef(opts.Ref),
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		for _, path := range opts.Paths {
			args = append(args, filepath.ToSlash(path))
		}
	}

	output, err := p.runGitOutput(ctx, workDir, args...)
	if err != nil {
		if isInvalidRefError(err) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRef, defaultRef(opts.Ref))
		}
		return nil, err
	}
	return parseGitLogOutput(output), nil
}

// GetWorkDir returns the working directory path for a workspace.
func (p *LocalProvider) GetWorkDir(ctx context.Context, workspaceID string) string {
	p.mu.RLock()
	if info, ok := p.workspaceIndex[workspaceID]; ok {
		p.mu.RUnlock()
		return info.workDir
	}
	p.mu.RUnlock()

	if p.workspaceSource != nil {
		if workDir, _, err := p.EnsureWorkspaceByID(ctx, workspaceID); err == nil {
			return workDir
		}
	}

	return ""
}

// RemoveWorkspace removes the workspace working directory.
func (p *LocalProvider) RemoveWorkspace(_ context.Context, workspaceID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, ok := p.workspaceIndex[workspaceID]
	if !ok {
		return nil
	}

	delete(p.workspaceIndex, workspaceID)
	return os.RemoveAll(info.workDir)
}

// ApplyPatches applies mbox-format patches (from git format-patch) to the workspace.
// Returns the final commit SHA after all patches are applied.
// If application fails, the operation is aborted without losing local changes.
func (p *LocalProvider) ApplyPatches(ctx context.Context, workspaceID string, patches []byte) (string, error) {
	workDir := p.GetWorkDir(ctx, workspaceID)
	if workDir == "" {
		return "", fmt.Errorf("%w: workspace %s", ErrNotFound, workspaceID)
	}

	if err := p.runGitWithStdin(ctx, workDir, patches, "apply", "--check"); err != nil {
		failedPatch := formatFailedPatchDiff(string(patches))
		if failedPatch != "" {
			return "", fmt.Errorf("patches will not apply cleanly: %w\n\nFailed patch diff:\n%s", err, failedPatch)
		}
		return "", fmt.Errorf("patches will not apply cleanly: %w", err)
	}

	if err := p.runGitWithStdin(ctx, workDir, patches, "am", "--keep-cr", "--no-gpg-sign"); err != nil {
		failedPatch := p.failedAmPatch(ctx, workDir)
		_ = p.runGit(ctx, workDir, "am", "--abort")
		if failedPatch != "" {
			return "", fmt.Errorf("failed to apply patches: %w\n\nFailed patch diff:\n%s", err, failedPatch)
		}
		return "", fmt.Errorf("failed to apply patches: %w", err)
	}

	finalCommit, err := p.runGitOutput(ctx, workDir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get final commit: %w", err)
	}
	return strings.TrimSpace(finalCommit), nil
}

func (p *LocalProvider) failedAmPatch(ctx context.Context, workDir string) string {
	patch, err := p.runGitOutput(ctx, workDir, "am", "--show-current-patch=diff")
	if err != nil {
		return ""
	}
	return formatFailedPatchDiff(patch)
}

func formatFailedPatchDiff(patch string) string {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return ""
	}

	if diffIndex := strings.Index(patch, "diff --git "); diffIndex >= 0 {
		patch = patch[diffIndex:]
	}

	const maxPatchLen = 12000
	if len(patch) > maxPatchLen {
		patch = patch[:maxPatchLen] + "\n... (truncated)"
	}
	return patch
}

func cleanGitEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return env
}

func gitEnv(extra map[string]string) []string {
	env := cleanGitEnv()
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}

func (p *LocalProvider) runGit(ctx context.Context, workDir string, args ...string) error {
	return p.runGitWithEnv(ctx, workDir, nil, args...)
}

func (p *LocalProvider) runGitWithEnv(ctx context.Context, workDir string, extraEnv map[string]string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = gitEnv(extraEnv)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}

func (p *LocalProvider) runGitWithStdin(ctx context.Context, workDir string, stdin []byte, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = cleanGitEnv()
	cmd.Stdin = bytes.NewReader(stdin)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}

func (p *LocalProvider) runGitOutput(ctx context.Context, workDir string, args ...string) (string, error) {
	output, err := p.runGitBytes(ctx, workDir, args...)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (p *LocalProvider) runGitBytes(ctx context.Context, workDir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = cleanGitEnv()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

// GetUserConfig retrieves the global git user name and email configuration.
func (p *LocalProvider) GetUserConfig(_ context.Context) (name, email string) {
	name = strings.TrimSpace(runGitConfigValue("--global", "user.name"))
	email = strings.TrimSpace(runGitConfigValue("--global", "user.email"))
	return name, email
}

func runGitConfigValue(scope, key string) string {
	cmd := exec.Command("git", "config", scope, "--get", key)
	cmd.Env = cleanGitEnv()
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
}

func (p *LocalProvider) currentCommit(ctx context.Context, workDir string) (string, error) {
	output, err := p.runGitOutput(ctx, workDir, "rev-parse", "HEAD")
	if err != nil {
		if isNotGitRepositoryError(err) {
			return "", ErrNotARepository
		}
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (p *LocalProvider) currentBranch(ctx context.Context, workDir string) (string, error) {
	output, err := p.runGitOutput(ctx, workDir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return strings.TrimSpace(output), nil
	}
	if _, headErr := p.currentCommit(ctx, workDir); headErr == nil {
		return "HEAD", nil
	}
	return "", err
}

func isNotGitRepositoryError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not a git repository")
}

func isInvalidRefError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "bad revision") ||
		strings.Contains(message, "unknown revision") ||
		strings.Contains(message, "ambiguous argument") ||
		strings.Contains(message, "invalid object name") ||
		strings.Contains(message, "needed a single revision") ||
		strings.Contains(message, "malformed object name")
}

func isGitPathNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "does not exist in") ||
		strings.Contains(message, "exists on disk, but not in") ||
		strings.Contains(message, "path '") && strings.Contains(message, " not in ")
}

func defaultRef(ref string) string {
	if ref == "" {
		return "HEAD"
	}
	return ref
}

func (p *LocalProvider) isGitRepositoryPath(ctx context.Context, path string) bool {
	output, err := p.runGitOutput(ctx, path, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(output) == "true"
}

func (p *LocalProvider) checkoutRef(ctx context.Context, workDir, ref string) error {
	if ref == "" {
		return nil
	}
	if err := p.runGit(ctx, workDir, "checkout", "--quiet", ref); err != nil {
		return err
	}
	return nil
}

func (p *LocalProvider) countAheadBehind(ctx context.Context, workDir string) (int, int) {
	output, err := p.runGitOutput(ctx, workDir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) < 2 {
		return 0, 0
	}
	ahead, _ := strconv.Atoi(fields[0])
	behind, _ := strconv.Atoi(fields[1])
	return ahead, behind
}

func isRenameOrCopyStatus(xy string) bool {
	return strings.ContainsRune(xy, 'R') || strings.ContainsRune(xy, 'C')
}

func isConflictStatus(xy string) bool {
	switch xy {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return strings.ContainsRune(xy, 'U')
	}
}

func parseGitDiffOutput(output string) []FileDiff {
	chunks := splitGitDiffChunks(output)
	diffs := make([]FileDiff, 0, len(chunks))
	for _, chunk := range chunks {
		diff, ok := parseGitDiffChunk(chunk)
		if ok {
			diffs = append(diffs, diff)
		}
	}
	return diffs
}

func splitGitDiffChunks(output string) []string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	chunks := []string{}
	current := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if len(current) > 0 {
				chunks = append(chunks, strings.Join(current, "\n"))
			}
			current = []string{line}
			continue
		}
		if len(current) > 0 {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, "\n"))
	}
	return chunks
}

func parseGitDiffChunk(chunk string) (FileDiff, bool) {
	lines := strings.Split(chunk, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "diff --git ") {
		return FileDiff{}, false
	}

	oldFile, newFile, ok := parseGitDiffHeaderPaths(lines[0])
	if !ok {
		return FileDiff{}, false
	}
	path := trimGitDiffPathPrefix(newFile)
	oldPath := ""
	status := "modified"
	binary := false

	for _, line := range lines[1:] {
		switch {
		case strings.HasPrefix(line, "rename from "):
			oldPath = strings.TrimPrefix(line, "rename from ")
			status = "renamed"
		case strings.HasPrefix(line, "rename to "):
			path = strings.TrimPrefix(line, "rename to ")
			status = "renamed"
		case strings.HasPrefix(line, "new file mode "):
			status = "added"
		case strings.HasPrefix(line, "deleted file mode "):
			status = "deleted"
		case strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch":
			binary = true
		}
	}

	if status == "deleted" {
		path = trimGitDiffPathPrefix(oldFile)
	}
	if status == "added" {
		oldPath = ""
	}
	if status == "modified" && oldPath != "" && oldPath != path {
		status = "renamed"
	}

	additions, deletions := 0, 0
	if !binary {
		additions, deletions = countPatchLines(chunk)
	}

	return FileDiff{
		Path:      path,
		OldPath:   oldPath,
		Status:    status,
		Binary:    binary,
		Additions: additions,
		Deletions: deletions,
		Patch:     chunk,
	}, true
}

func parseGitDiffHeaderPaths(line string) (string, string, bool) {
	rest := strings.TrimPrefix(line, "diff --git ")
	if strings.HasPrefix(rest, "\"") {
		first, remaining, ok := consumeQuotedPath(rest)
		if !ok {
			return "", "", false
		}
		second, _, ok := consumeQuotedPath(strings.TrimLeft(remaining, " "))
		if !ok {
			return "", "", false
		}
		return first, second, true
	}

	parts := strings.SplitN(rest, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func consumeQuotedPath(input string) (string, string, bool) {
	if !strings.HasPrefix(input, "\"") {
		return "", "", false
	}
	escaped := false
	for i := 1; i < len(input); i++ {
		switch {
		case escaped:
			escaped = false
		case input[i] == '\\':
			escaped = true
		case input[i] == '"':
			value, err := strconv.Unquote(input[:i+1])
			if err != nil {
				return "", "", false
			}
			return value, input[i+1:], true
		}
	}
	return "", "", false
}

func trimGitDiffPathPrefix(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

func parseGitLogOutput(output string) []Commit {
	records := strings.Split(output, "\x1e")
	commits := make([]Commit, 0, len(records))
	for _, record := range records {
		record = strings.Trim(record, "\n\x00")
		if record == "" {
			continue
		}
		fields := strings.Split(record, "\x1f")
		if len(fields) < 9 {
			continue
		}
		authorDate, _ := time.Parse(time.RFC3339, fields[5])
		commitDate, _ := time.Parse(time.RFC3339, fields[7])
		message := strings.TrimSuffix(fields[2], "\n")
		commits = append(commits, Commit{
			SHA:         fields[0],
			ShortSHA:    fields[1],
			Message:     message,
			Author:      fields[3],
			AuthorEmail: fields[4],
			AuthorDate:  authorDate,
			Committer:   fields[6],
			CommitDate:  commitDate,
			Parents:     strings.Fields(fields[8]),
		})
	}
	return commits
}

func countPatchLines(patch string) (int, int) {
	additions := 0
	deletions := 0
	for line := range strings.SplitSeq(patch, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}
	return additions, deletions
}

func statusCodeToString(code byte) string {
	switch code {
	case 'A':
		return "added"
	case 'M', 'T':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'U':
		return "unmerged"
	default:
		return "unknown"
	}
}

func (p *LocalProvider) getCommit(ctx context.Context, workDir, ref string) (*Commit, error) {
	commits, err := p.logAtRef(ctx, workDir, ref, 1)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, ref)
	}
	return &commits[0], nil
}

func (p *LocalProvider) logAtRef(ctx context.Context, workDir, ref string, limit int) ([]Commit, error) {
	output, err := p.runGitOutput(ctx, workDir,
		"log",
		fmt.Sprintf("--max-count=%d", limit),
		"--date=iso-strict",
		"--format=%H%x1f%h%x1f%B%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%cI%x1f%P%x1e",
		defaultRef(ref),
	)
	if err != nil {
		if isInvalidRefError(err) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRef, defaultRef(ref))
		}
		return nil, err
	}
	return parseGitLogOutput(output), nil
}

func (p *LocalProvider) loadCommitterIdentity(ctx context.Context, workDir string) (string, string) {
	lookup := func(key string) string {
		value, err := p.runGitOutput(ctx, workDir, "config", "--get", key)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(value)
	}

	name := lookup("committer.name")
	email := lookup("committer.email")
	if name != "" && email != "" {
		return name, email
	}

	name = lookup("user.name")
	email = lookup("user.email")
	if name != "" && email != "" {
		return name, email
	}

	return p.GetUserConfig(ctx)
}

// Ensure LocalProvider implements Provider.
var _ Provider = (*LocalProvider)(nil)
