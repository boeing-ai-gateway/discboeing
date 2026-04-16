// Package gitops provides git operations for the agent API (diff, format-patch).
package gitops

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FileDiffEntry represents a single changed file in the diff.
type FileDiffEntry struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // "added", "modified", "deleted", "renamed"
	OldPath   string `json:"oldPath,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Binary    bool   `json:"binary"`
	Patch     string `json:"patch,omitempty"`
}

// DiffStats contains summary statistics for a diff.
type DiffStats struct {
	FilesChanged int `json:"filesChanged"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}

// DiffResult is the full diff result.
type DiffResult struct {
	Files []FileDiffEntry `json:"files"`
	Stats DiffStats       `json:"stats"`
}

// CommitsResult is the successful result of getting commit patches.
type CommitsResult struct {
	Patches     string `json:"patches"`
	CommitCount int    `json:"commitCount"`
	HeadCommit  string `json:"headCommit"`
}

// CommitsError represents an error during commit operations.
type CommitsError struct {
	Code       string // "invalid_target", "not_git_repo", "no_commits"
	Message    string
	IsClean    bool   // populated for "no_commits": true when working tree has no uncommitted changes
	HeadCommit string // populated for "no_commits": the current HEAD commit SHA
}

func (e *CommitsError) Error() string {
	return e.Message
}

// IsGitRepo checks if the directory is a git repository.
func IsGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// IsWorkingTreeClean returns true when there are no staged, unstaged, or untracked changes.
func IsWorkingTreeClean(workspaceRoot string) bool {
	out, err := gitCmd(workspaceRoot, "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) == ""
}

// HeadCommitSHA returns the current HEAD commit SHA for the repository.
func HeadCommitSHA(workspaceRoot string) string {
	out, err := gitCmd(workspaceRoot, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// gitCmd runs a git command in the given directory and returns stdout.
func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func gitCmdCombined(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func gitCmdCombinedEnv(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func defaultDiffTarget(workspaceRoot string) string {
	upstream, err := gitCmd(workspaceRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil || strings.TrimSpace(upstream) == "" {
		return "HEAD"
	}
	mergeBase, err := gitCmd(workspaceRoot, "merge-base", "HEAD", strings.TrimSpace(upstream))
	if err != nil || strings.TrimSpace(mergeBase) == "" {
		return "HEAD"
	}
	return strings.TrimSpace(mergeBase)
}

// parseDiffOutput parses unified diff output into structured entries.
func parseDiffOutput(output string) DiffResult {
	var files []FileDiffEntry
	var current *FileDiffEntry
	var patchLines []string

	for line := range strings.SplitSeq(output, "\n") {
		// Check for diff header
		if strings.HasPrefix(line, "diff --git a/") {
			// Save previous entry
			if current != nil {
				current.Patch = strings.Join(patchLines, "\n")
				files = append(files, *current)
			}

			// Parse "diff --git a/oldPath b/newPath"
			parts := strings.SplitN(line, " b/", 2)
			oldPath := strings.TrimPrefix(parts[0], "diff --git a/")
			newPath := oldPath
			if len(parts) == 2 {
				newPath = parts[1]
			}

			current = &FileDiffEntry{
				Path:   newPath,
				Status: "modified",
			}
			if oldPath != newPath {
				current.OldPath = oldPath
			}
			patchLines = []string{line}
			continue
		}

		if current != nil {
			patchLines = append(patchLines, line)

			switch {
			case strings.HasPrefix(line, "new file mode"):
				current.Status = "added"
			case strings.HasPrefix(line, "deleted file mode"):
				current.Status = "deleted"
			case strings.HasPrefix(line, "rename from"):
				current.Status = "renamed"
			case strings.HasPrefix(line, "Binary files"):
				current.Binary = true
			case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
				current.Additions++
			case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
				current.Deletions++
			}
		}
	}

	// Don't forget the last entry
	if current != nil {
		current.Patch = strings.Join(patchLines, "\n")
		files = append(files, *current)
	}

	stats := DiffStats{FilesChanged: len(files)}
	for _, f := range files {
		stats.Additions += f.Additions
		stats.Deletions += f.Deletions
	}

	return DiffResult{Files: files, Stats: stats}
}

// getUntrackedFiles returns a list of untracked files.
func getUntrackedFiles(dir string) []string {
	out, err := gitCmd(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil || out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// isBinaryContent checks if content contains null bytes (likely binary).
func isBinaryContent(data []byte) bool {
	checkLen := min(len(data), 8000)
	for i := range checkLen {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// getUntrackedFileDiff creates a synthetic diff entry for an untracked file.
func getUntrackedFileDiff(dir, filePath string) FileDiffEntry {
	fullPath := filepath.Join(dir, filePath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return FileDiffEntry{
			Path:   filePath,
			Status: "added",
			Patch:  fmt.Sprintf("diff --git a/%s b/%s\nnew file mode 100644", filePath, filePath),
		}
	}

	if isBinaryContent(data) {
		return FileDiffEntry{
			Path:   filePath,
			Status: "added",
			Binary: true,
			Patch:  fmt.Sprintf("diff --git a/%s b/%s\nnew file mode 100644\nBinary file", filePath, filePath),
		}
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	// Handle trailing newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	additions := len(lines)

	var patchLines []string
	patchLines = append(patchLines, fmt.Sprintf("diff --git a/%s b/%s", filePath, filePath))
	patchLines = append(patchLines, "new file mode 100644")
	patchLines = append(patchLines, "--- /dev/null")
	patchLines = append(patchLines, fmt.Sprintf("+++ b/%s", filePath))
	patchLines = append(patchLines, fmt.Sprintf("@@ -0,0 +1,%d @@", additions))
	for _, line := range lines {
		patchLines = append(patchLines, "+"+line)
	}

	return FileDiffEntry{
		Path:      filePath,
		Status:    "added",
		Additions: additions,
		Patch:     strings.Join(patchLines, "\n"),
	}
}

// GetDiff returns the workspace diff relative to a target commit or ref.
// When target is empty, the sandbox computes a local merge-base against the
// tracked upstream when available and falls back to HEAD. Untracked files are
// included separately.
func GetDiff(workspaceRoot, singlePath, target string) (DiffResult, error) {
	if !IsGitRepo(workspaceRoot) {
		return DiffResult{
			Files: []FileDiffEntry{},
			Stats: DiffStats{},
		}, nil
	}

	target = strings.TrimSpace(target)
	if target == "" {
		target = defaultDiffTarget(workspaceRoot)
	}

	if _, err := gitCmd(workspaceRoot, "cat-file", "-e", target+"^{commit}"); err != nil {
		return DiffResult{}, fmt.Errorf("target %q does not exist in repository", target)
	}

	// Build git diff command
	args := []string{"diff", "--no-color", target}
	if singlePath != "" {
		args = append(args, "--", singlePath)
	}

	var trackedDiff DiffResult
	out, err := gitCmd(workspaceRoot, args...)
	if err != nil {
		// git diff may return exit code 1 when there are differences
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Try to get stdout from combined output
			cmd := exec.Command("git", args...)
			cmd.Dir = workspaceRoot
			stdout, _ := cmd.Output()
			trackedDiff = parseDiffOutput(string(stdout))
		} else {
			return DiffResult{}, fmt.Errorf("git diff failed: %w", err)
		}
	} else {
		trackedDiff = parseDiffOutput(out)
	}

	// Get untracked files
	untrackedFiles := getUntrackedFiles(workspaceRoot)

	// Filter untracked files if looking for a single path
	if singlePath != "" {
		filtered := make([]string, 0)
		for _, f := range untrackedFiles {
			if f == singlePath {
				filtered = append(filtered, f)
			}
		}
		untrackedFiles = filtered
	}

	// Build diff entries for untracked files
	for _, f := range untrackedFiles {
		trackedDiff.Files = append(trackedDiff.Files, getUntrackedFileDiff(workspaceRoot, f))
	}

	// Recalculate stats
	trackedDiff.Stats = DiffStats{FilesChanged: len(trackedDiff.Files)}
	for _, f := range trackedDiff.Files {
		trackedDiff.Stats.Additions += f.Additions
		trackedDiff.Stats.Deletions += f.Deletions
	}

	return trackedDiff, nil
}

func defaultCommitTarget(workspaceRoot string) string {
	upstream, err := gitCmd(workspaceRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil || strings.TrimSpace(upstream) == "" {
		return "HEAD"
	}
	mergeBase, err := gitCmd(workspaceRoot, "merge-base", "HEAD", strings.TrimSpace(upstream))
	if err != nil || strings.TrimSpace(mergeBase) == "" {
		return "HEAD"
	}
	return strings.TrimSpace(mergeBase)
}

// GetCommitPatches returns format-patch output for changes relative to a target
// commit. When target is empty, the sandbox computes a local merge-base against
// the tracked upstream when available and falls back to HEAD. If the target is
// an ancestor of HEAD, the existing commit series is preserved. Otherwise a
// synthetic single-commit patch is generated from the target tree to the
// current HEAD tree.
func GetCommitPatches(workspaceRoot, target string) (*CommitsResult, *CommitsError) {
	if !IsGitRepo(workspaceRoot) {
		return nil, &CommitsError{Code: "not_git_repo", Message: "Workspace is not a git repository"}
	}

	target = strings.TrimSpace(target)
	if target == "" {
		target = defaultCommitTarget(workspaceRoot)
	}

	if _, err := gitCmd(workspaceRoot, "cat-file", "-e", target+"^{commit}"); err != nil {
		return nil, &CommitsError{Code: "invalid_target", Message: fmt.Sprintf("Target commit %s does not exist in repository", target)}
	}

	headCommit := HeadCommitSHA(workspaceRoot)
	if headCommit == "" {
		return nil, &CommitsError{
			Code:       "no_commits",
			Message:    fmt.Sprintf("No changes found between %s and HEAD", target),
			IsClean:    IsWorkingTreeClean(workspaceRoot),
			HeadCommit: headCommit,
		}
	}

	diffExists, err := diffExistsBetweenCommits(workspaceRoot, target, "HEAD")
	if err != nil {
		return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to compare %s and HEAD: %v", target, err)}
	}
	if !diffExists {
		return nil, &CommitsError{
			Code:       "no_commits",
			Message:    fmt.Sprintf("No changes found between %s and HEAD", target),
			IsClean:    IsWorkingTreeClean(workspaceRoot),
			HeadCommit: headCommit,
		}
	}

	isAncestor, err := isAncestorCommit(workspaceRoot, target, "HEAD")
	if err != nil {
		return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to inspect commit ancestry: %v", err)}
	}
	if isAncestor {
		countStr, err := gitCmd(workspaceRoot, "rev-list", "--count", target+"..HEAD")
		if err != nil {
			return nil, &CommitsError{
				Code:       "no_commits",
				Message:    "Failed to count commits",
				IsClean:    IsWorkingTreeClean(workspaceRoot),
				HeadCommit: headCommit,
			}
		}
		commitCount, err := strconv.Atoi(countStr)
		if err != nil || commitCount == 0 {
			return nil, &CommitsError{
				Code:       "no_commits",
				Message:    fmt.Sprintf("No changes found between %s and HEAD", target),
				IsClean:    IsWorkingTreeClean(workspaceRoot),
				HeadCommit: headCommit,
			}
		}

		patches, err := gitCmd(workspaceRoot, "format-patch", "--stdout", target+"..HEAD")
		if err != nil {
			return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to generate patches: %v", err)}
		}
		return &CommitsResult{Patches: patches, CommitCount: commitCount, HeadCommit: headCommit}, nil
	}

	patches, err := synthesizePatchAgainstTarget(workspaceRoot, target)
	if err != nil {
		return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to synthesize patches against %s: %v", target, err)}
	}
	return &CommitsResult{Patches: patches, CommitCount: 1, HeadCommit: headCommit}, nil
}

func diffExistsBetweenCommits(workspaceRoot, base, head string) (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet", "--binary", base, head)
	cmd.Dir = workspaceRoot
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}

func isAncestorCommit(workspaceRoot, ancestor, descendant string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = workspaceRoot
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func synthesizePatchAgainstTarget(workspaceRoot, target string) (string, error) {
	headMetadata, err := readHeadCommitMetadata(workspaceRoot)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "discobot-target-patch-*")
	if err != nil {
		return "", fmt.Errorf("create temp worktree dir: %w", err)
	}
	defer func() {
		_, _ = gitCmdCombined(workspaceRoot, "worktree", "remove", "--force", tmpDir)
		_ = os.RemoveAll(tmpDir)
	}()

	if _, err := gitCmdCombined(workspaceRoot, "worktree", "add", "--detach", tmpDir, target); err != nil {
		return "", fmt.Errorf("create temp worktree: %w", err)
	}

	diffOutput, err := gitCmd(workspaceRoot, "diff", "--binary", target, "HEAD")
	if err != nil {
		return "", fmt.Errorf("generate target diff: %w", err)
	}

	patchPath := filepath.Join(tmpDir, ".discobot-target.patch")
	if err := os.WriteFile(patchPath, []byte(diffOutput), 0600); err != nil {
		return "", fmt.Errorf("write target diff: %w", err)
	}
	if _, err := gitCmdCombined(tmpDir, "apply", "--index", "--binary", patchPath); err != nil {
		return "", fmt.Errorf("apply target diff: %w", err)
	}

	messagePath := filepath.Join(tmpDir, ".discobot-target-message.txt")
	if err := os.WriteFile(messagePath, []byte(headMetadata.message), 0600); err != nil {
		return "", fmt.Errorf("write synthetic commit message: %w", err)
	}

	env := []string{
		"GIT_AUTHOR_NAME=" + headMetadata.authorName,
		"GIT_AUTHOR_EMAIL=" + headMetadata.authorEmail,
		"GIT_AUTHOR_DATE=" + headMetadata.authorDate,
		"GIT_COMMITTER_NAME=" + headMetadata.committerName,
		"GIT_COMMITTER_EMAIL=" + headMetadata.committerEmail,
		"GIT_COMMITTER_DATE=" + headMetadata.committerDate,
	}
	if _, err := gitCmdCombinedEnv(tmpDir, env, "commit", "--allow-empty", "--file", messagePath); err != nil {
		return "", fmt.Errorf("create synthetic commit: %w", err)
	}

	patches, err := gitCmdCombined(tmpDir, "format-patch", "--stdout", target+"..HEAD")
	if err != nil {
		return "", fmt.Errorf("format synthetic patch: %w", err)
	}
	return patches, nil
}

type commitMetadata struct {
	message        string
	authorName     string
	authorEmail    string
	authorDate     string
	committerName  string
	committerEmail string
	committerDate  string
}

func readHeadCommitMetadata(workspaceRoot string) (*commitMetadata, error) {
	out, err := gitCmd(workspaceRoot, "show", "-s", "--format=%B%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("read HEAD commit metadata: %w", err)
	}
	parts := strings.Split(out, "\x00")
	if len(parts) != 7 {
		return nil, fmt.Errorf("unexpected HEAD commit metadata format")
	}
	metadata := &commitMetadata{
		message:        strings.TrimRight(parts[0], "\n"),
		authorName:     strings.TrimSpace(parts[1]),
		authorEmail:    strings.TrimSpace(parts[2]),
		authorDate:     strings.TrimSpace(parts[3]),
		committerName:  strings.TrimSpace(parts[4]),
		committerEmail: strings.TrimSpace(parts[5]),
		committerDate:  strings.TrimSpace(parts[6]),
	}
	if metadata.message == "" {
		metadata.message = "Discobot synthetic patch"
	}
	if metadata.committerName == "" {
		metadata.committerName = metadata.authorName
	}
	if metadata.committerEmail == "" {
		metadata.committerEmail = metadata.authorEmail
	}
	if metadata.committerDate == "" {
		metadata.committerDate = metadata.authorDate
	}
	return metadata, nil
}
