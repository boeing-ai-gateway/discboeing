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

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
)

// FileDiffEntry represents a single changed file in the diff.
type FileDiffEntry = api.FileDiffEntry

// DiffStats contains summary statistics for a diff.
type DiffStats = api.DiffStats

// DiffResult is the full diff result.
type DiffResult = api.DiffResponse

// CommitsResult is the successful result of getting commit patches.
type CommitsResult = api.CommitsResponse

// WorkspaceChangeCommit describes a Discboeing workspace change commit.
type WorkspaceChangeCommit = api.WorkspaceChangeCommit

// WorkspaceChangeCommitsResult is the successful result of listing workspace change commits.
type WorkspaceChangeCommitsResult = api.WorkspaceChangeCommitsResponse

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

// ListWorkspaceChangeCommits returns Discboeing workspace change commits for one session. Results
// are sorted newest-first by committer date.
const workspaceChangeCommitRefPrefix = "refs/discboeing/workspace-change-commits"

func ListWorkspaceChangeCommits(workspaceRoot, sessionID string) (*WorkspaceChangeCommitsResult, error) {
	if _, err := gitCmdCombined(workspaceRoot, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, &CommitsError{Code: "not_git_repo", Message: "Not a git repository"}
	}
	refGlob := workspaceChangeCommitRefPrefix
	if strings.TrimSpace(sessionID) != "" {
		refGlob = workspaceChangeCommitRefPrefix + "/" + sanitizeRefGlobPart(sessionID)
	}
	out, err := gitCmdCombined(workspaceRoot, "for-each-ref",
		"--sort=-committerdate",
		"--format=%(objectname)%00%(committerdate:iso-strict)",
		refGlob,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspace change commit refs: %w", err)
	}
	result := &WorkspaceChangeCommitsResult{Commits: []WorkspaceChangeCommit{}}
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		hash := strings.TrimSpace(parts[0])
		stat, err := commitDiffStat(workspaceRoot, hash)
		if err != nil {
			return nil, err
		}
		result.Commits = append(result.Commits, WorkspaceChangeCommit{
			CreatedAt: strings.TrimSpace(parts[1]),
			Hash:      hash,
			DiffStat:  stat,
		})
	}
	return result, nil
}

func commitDiffStat(workspaceRoot, hash string) (DiffStats, error) {
	out, err := gitCmdCombined(workspaceRoot, "diff", "--numstat", hash+"^", hash)
	if err != nil {
		return DiffStats{}, fmt.Errorf("diffstat for workspace change commit %s: %w", hash, err)
	}
	stats := DiffStats{}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		stats.FilesChanged++
		if fields[0] != "-" {
			additions, _ := strconv.Atoi(fields[0])
			stats.Additions += additions
		}
		if fields[1] != "-" {
			deletions, _ := strconv.Atoi(fields[1])
			stats.Deletions += deletions
		}
	}
	return stats, nil
}

func sanitizeRefGlobPart(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
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
	targetCommit, err := gitCmd(workspaceRoot, "rev-parse", target+"^{commit}")
	if err != nil || strings.TrimSpace(targetCommit) == "" {
		return DiffResult{}, fmt.Errorf("target commit %s does not exist in repository", target)
	}
	targetCommit = strings.TrimSpace(targetCommit)
	if isWorkspaceChangeCommit(workspaceRoot, targetCommit) {
		return DiffResult{}, &CommitsError{Code: "invalid_target", Message: "Workspace change commits cannot be rendered as diffs"}
	}
	target = targetCommit

	// Build git diff command
	args := []string{"diff", "--no-color", target}
	if singlePath != "" {
		args = append(args, "--", singlePath)
	}

	var trackedDiff DiffResult
	out, err := gitCmd(workspaceRoot, args...)
	if err != nil {
		// git diff may return exit code 1 when there are differences
		exitErr := new(exec.ExitError)
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
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

func defaultCommitTarget(workspaceRoot, headCommit string) string {
	candidates := []string{}
	if upstream, err := gitCmd(workspaceRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil && strings.TrimSpace(upstream) != "" {
		candidates = append(candidates, strings.TrimSpace(upstream))
	}
	if originHead, err := gitCmd(workspaceRoot, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil && strings.TrimSpace(originHead) != "" {
		candidates = append(candidates, strings.TrimSpace(originHead))
	}
	candidates = append(candidates,
		"refs/remotes/origin/main",
		"refs/remotes/origin/master",
		"refs/heads/main",
		"refs/heads/master",
	)

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := gitCmd(workspaceRoot, "rev-parse", "--verify", candidate+"^{commit}"); err != nil {
			continue
		}
		mergeBase, err := gitCmd(workspaceRoot, "merge-base", headCommit, candidate)
		if err != nil || strings.TrimSpace(mergeBase) == "" {
			continue
		}
		mergeBase = strings.TrimSpace(mergeBase)
		if mergeBase != headCommit {
			return mergeBase
		}
	}

	if parentCommit, err := gitCmd(workspaceRoot, "rev-parse", headCommit+"^"); err == nil && strings.TrimSpace(parentCommit) != "" {
		return strings.TrimSpace(parentCommit)
	}
	return headCommit
}

// GetCommitPatches returns format-patch output for changes relative to a target
// commit and the current HEAD commit. When target is empty, the sandbox
// computes a local merge-base against the tracked upstream when available and
// falls back to a best-effort base for HEAD. If the target is an ancestor of
// HEAD, the existing commit series is preserved. Otherwise a synthetic
// single-commit patch is generated from the target tree to the current HEAD
// tree.
func GetCommitPatches(workspaceRoot, target string) (*CommitsResult, *CommitsError) {
	return GetCommitPatchesAtHead(workspaceRoot, target, "")
}

// GetCommitPatchesAtHead returns format-patch output for changes between a base
// commit and an explicit tip commit within the given git working directory.
// When head is empty, the current HEAD commit is used. When target is empty, the
// sandbox derives a best-effort base commit for the requested head.
func GetCommitPatchesAtHead(workspaceRoot, target, head string) (*CommitsResult, *CommitsError) {
	if !IsGitRepo(workspaceRoot) {
		return nil, &CommitsError{Code: "not_git_repo", Message: "Workspace is not a git repository"}
	}

	head = strings.TrimSpace(head)
	if head == "" {
		head = "HEAD"
	}
	headCommit, err := gitCmd(workspaceRoot, "rev-parse", head+"^{commit}")
	if err != nil || strings.TrimSpace(headCommit) == "" {
		return nil, &CommitsError{Code: "invalid_target", Message: fmt.Sprintf("Head commit %s does not exist in repository", strings.TrimSpace(head))}
	}
	headCommit = strings.TrimSpace(headCommit)
	if isWorkspaceChangeCommit(workspaceRoot, headCommit) {
		return nil, &CommitsError{Code: "invalid_target", Message: "Workspace change commits cannot be exported as patches"}
	}

	target = strings.TrimSpace(target)
	if target == "" {
		target = defaultCommitTarget(workspaceRoot, headCommit)
	}

	targetCommit, err := gitCmd(workspaceRoot, "rev-parse", target+"^{commit}")
	if err != nil || strings.TrimSpace(targetCommit) == "" {
		return nil, &CommitsError{Code: "invalid_target", Message: fmt.Sprintf("Target commit %s does not exist in repository", target)}
	}
	targetCommit = strings.TrimSpace(targetCommit)
	if isWorkspaceChangeCommit(workspaceRoot, targetCommit) {
		return nil, &CommitsError{Code: "invalid_target", Message: "Workspace change commits cannot be exported as patches"}
	}
	target = targetCommit

	diffExists, err := diffExistsBetweenCommits(workspaceRoot, target, headCommit)
	if err != nil {
		return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to compare %s and %s: %v", target, headCommit, err), IsClean: IsWorkingTreeClean(workspaceRoot), HeadCommit: headCommit}
	}
	if !diffExists {
		return nil, &CommitsError{
			Code:       "no_commits",
			Message:    fmt.Sprintf("No changes found between %s and %s", target, headCommit),
			IsClean:    IsWorkingTreeClean(workspaceRoot),
			HeadCommit: headCommit,
		}
	}

	isAncestor, err := isAncestorCommit(workspaceRoot, target, headCommit)
	if err != nil {
		return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to inspect commit ancestry: %v", err), IsClean: IsWorkingTreeClean(workspaceRoot), HeadCommit: headCommit}
	}
	if isAncestor {
		countStr, err := gitCmd(workspaceRoot, "rev-list", "--count", target+".."+headCommit)
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
				Message:    fmt.Sprintf("No changes found between %s and %s", target, headCommit),
				IsClean:    IsWorkingTreeClean(workspaceRoot),
				HeadCommit: headCommit,
			}
		}

		patches, err := gitCmd(workspaceRoot, "format-patch", "--stdout", target+".."+headCommit)
		if err != nil {
			return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to generate patches: %v", err)}
		}
		return &CommitsResult{Patches: patches, CommitCount: commitCount, HeadCommit: headCommit}, nil
	}

	patches, err := synthesizePatchAgainstTarget(workspaceRoot, target, headCommit)
	if err != nil {
		return nil, &CommitsError{Code: "no_commits", Message: fmt.Sprintf("Failed to synthesize patches against %s: %v", target, err)}
	}
	return &CommitsResult{Patches: patches, CommitCount: 1, HeadCommit: headCommit}, nil
}

func isWorkspaceChangeCommit(workspaceRoot, commit string) bool {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return false
	}
	out, err := gitCmdCombined(workspaceRoot, "for-each-ref", "--format=%(objectname)", workspaceChangeCommitRefPrefix)
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) == commit {
			return true
		}
	}
	return false
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

func synthesizePatchAgainstTarget(workspaceRoot, target, headCommit string) (string, error) {
	headMetadata, err := readCommitMetadata(workspaceRoot, headCommit)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "discboeing-target-patch-*")
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

	diffOutput, err := gitCmd(workspaceRoot, "diff", "--binary", target, headCommit)
	if err != nil {
		return "", fmt.Errorf("generate target diff: %w", err)
	}

	patchPath := filepath.Join(tmpDir, ".discboeing-target.patch")
	if err := os.WriteFile(patchPath, []byte(diffOutput), 0600); err != nil {
		return "", fmt.Errorf("write target diff: %w", err)
	}
	if _, err := gitCmdCombined(tmpDir, "apply", "--index", "--binary", patchPath); err != nil {
		return "", fmt.Errorf("apply target diff: %w", err)
	}

	messagePath := filepath.Join(tmpDir, ".discboeing-target-message.txt")
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

func readCommitMetadata(workspaceRoot, commit string) (*commitMetadata, error) {
	out, err := gitCmd(workspaceRoot, "show", "-s", "--format=%B%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI", commit)
	if err != nil {
		return nil, fmt.Errorf("read commit metadata for %s: %w", strings.TrimSpace(commit), err)
	}
	parts := strings.Split(out, "\x00")
	if len(parts) != 7 {
		return nil, fmt.Errorf("unexpected commit metadata format")
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
		metadata.message = "Discboeing synthetic patch"
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
