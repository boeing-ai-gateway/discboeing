package tools

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"` // optional directory to search in
}

// globSkipDirs are directory names that are always skipped during glob walks.
// This mirrors the skip logic in internal/grep/walker.go.
var globSkipDirs = map[string]bool{
	"node_modules": true,
	"__pycache__":  true,
	".git":         true,
}

func (e *Executor) executeGlob(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input globInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.Pattern == "" {
		return errResult(call, "pattern is required"), nil
	}

	// Validate the pattern before walking.
	if !doublestar.ValidatePattern(input.Pattern) {
		return errResult(call, "invalid glob pattern"), nil
	}

	// Determine search root.
	root := e.cwd
	if input.Path != "" {
		root = resolvePath(e.cwd, input.Path)
	}

	// For absolute patterns, extract the relative portion against root if
	// possible, otherwise fall back to the pattern as-is.
	matchPattern := input.Pattern
	if filepath.IsAbs(input.Pattern) {
		rel, relErr := filepath.Rel(root, input.Pattern)
		if relErr == nil {
			matchPattern = rel
		}
	}

	// Walk the filesystem using filepath.WalkDir, which does not follow
	// symbolic links. This avoids traversing node_modules symlinks (and
	// similar) that would otherwise flood results with thousands of
	// irrelevant package files.
	var matched []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil {
			if d.IsDir() {
				name := d.Name()
				// Never skip the root itself, even if it's a hidden directory.
				if path == root {
					return nil
				}
				// Always skip well-known non-source directories (node_modules, .git, etc.).
				if globSkipDirs[name] {
					return filepath.SkipDir
				}
				// Skip hidden directories unless the pattern explicitly targets them.
				// For example, ".discboeing/**/*" should descend into ".discboeing".
				if strings.HasPrefix(name, ".") {
					rel, relErr := filepath.Rel(root, path)
					if relErr != nil {
						return filepath.SkipDir
					}
					rel = filepath.ToSlash(rel)
					if !strings.HasPrefix(matchPattern, rel+"/") && matchPattern != rel {
						return filepath.SkipDir
					}
				}
				return nil
			}
			// Match the relative path against the pattern.
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil {
				// Normalize to forward slashes for consistent cross-platform output.
				rel = filepath.ToSlash(rel)
				ok, matchErr := doublestar.Match(matchPattern, rel)
				if matchErr == nil && ok {
					matched = append(matched, rel)
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return errToolResult(call, "error walking directory: "+walkErr.Error())
	}

	if len(matched) == 0 {
		return textResult(call, "No files found"), nil
	}

	sort.Strings(matched)

	var sb strings.Builder
	for _, p := range matched {
		sb.WriteString(p)
		sb.WriteByte('\n')
	}

	return textResult(call, strings.TrimRight(sb.String(), "\n")), nil
}
