package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

type editInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

type editLine struct {
	text  string
	start int
	end   int
}

func (e *Executor) executeEdit(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input editInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.FilePath == "" {
		return errResult(call, "file_path is required"), nil
	}
	if input.OldString == "" {
		return errResult(call, "old_string is required"), nil
	}

	path := resolvePath(e.cwd, input.FilePath)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errResult(call, fmt.Sprintf("file not found: %s", input.FilePath)), nil
		}
		return errResult(call, err.Error()), nil
	}

	if err := validateToolReadableTextFile(data, input.FilePath); err != nil {
		return errResult(call, err.Error()), nil
	}
	if err := validateToolWriteTextContent(input.NewString, input.FilePath); err != nil {
		return errResult(call, err.Error()), nil
	}

	content := string(data)
	count := strings.Count(content, input.OldString)
	if count == 0 {
		matches := findLenientEditMatches(content, input.OldString)
		if len(matches) == 0 {
			return errResult(call, fmt.Sprintf("old_string not found in %s.", input.FilePath)), nil
		}
		if len(matches) > 1 && !input.ReplaceAll {
			return errResult(call, fmt.Sprintf(
				"old_string matches %d locations in %s after whitespace normalization. Use replace_all: true to replace all occurrences, or provide more context to make it unique.",
				len(matches), input.FilePath,
			)), nil
		}

		newContent := applyLenientEditMatches(content, matches, input.NewString, input.ReplaceAll)
		if err := validateToolWriteTextContent(newContent, input.FilePath); err != nil {
			return errResult(call, err.Error()), nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return errResult(call, fmt.Sprintf("failed to create parent directory: %v", err)), nil
		}
		if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
			return errResult(call, fmt.Sprintf("failed to write file: %v", err)), nil
		}

		e.recordFileWritten(path)
		if input.ReplaceAll && len(matches) > 1 {
			return textResult(call, fmt.Sprintf("Successfully replaced %d occurrences in %s", len(matches), input.FilePath)), nil
		}
		return textResult(call, fmt.Sprintf("Successfully edited %s", input.FilePath)), nil
	}

	if count > 1 && !input.ReplaceAll {
		return errResult(call, fmt.Sprintf(
			"old_string appears %d times in %s. Use replace_all: true to replace all occurrences, or provide more context to make it unique.",
			count, input.FilePath,
		)), nil
	}

	var newContent string
	if input.ReplaceAll {
		newContent = strings.ReplaceAll(content, input.OldString, input.NewString)
	} else {
		newContent = strings.Replace(content, input.OldString, input.NewString, 1)
	}
	if err := validateToolWriteTextContent(newContent, input.FilePath); err != nil {
		return errResult(call, err.Error()), nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errResult(call, fmt.Sprintf("failed to create parent directory: %v", err)), nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return errResult(call, fmt.Sprintf("failed to write file: %v", err)), nil
	}

	e.recordFileWritten(path)

	if input.ReplaceAll && count > 1 {
		return textResult(call, fmt.Sprintf("Successfully replaced %d occurrences in %s", count, input.FilePath)), nil
	}
	return textResult(call, fmt.Sprintf("Successfully edited %s", input.FilePath)), nil
}

func findLenientEditMatches(content, oldString string) []editLine {
	contentLines := splitEditLines(content)
	patternLines := splitEditPatternLines(oldString)
	if len(contentLines) == 0 || len(patternLines) == 0 || len(patternLines) > len(contentLines) {
		return nil
	}

	for _, matcher := range []func([]string, []string, int) bool{
		patchTrimEndMatch,
		patchTrimMatch,
		patchNormalizedMatch,
	} {
		matches := make([]editLine, 0)
		for start := 0; start <= len(contentLines)-len(patternLines); start++ {
			candidate := make([]string, len(patternLines))
			for i := range patternLines {
				candidate[i] = contentLines[start+i].text
			}
			if !matcher(candidate, patternLines, 0) {
				continue
			}
			matches = append(matches, editLine{
				start: contentLines[start].start,
				end:   contentLines[start+len(patternLines)-1].end,
			})
		}
		if len(matches) > 0 {
			return matches
		}
	}

	return nil
}

func applyLenientEditMatches(content string, matches []editLine, newString string, replaceAll bool) string {
	if len(matches) == 0 {
		return content
	}
	if !replaceAll {
		match := matches[0]
		return content[:match.start] + newString + content[match.end:]
	}

	updated := content
	for _, v := range slices.Backward(matches) {
		match := v
		updated = updated[:match.start] + newString + updated[match.end:]
	}
	return updated
}

func splitEditPatternLines(s string) []string {
	lines := splitEditLines(s)
	out := make([]string, len(lines))
	for i := range lines {
		out[i] = lines[i].text
	}
	return out
}

func splitEditLines(s string) []editLine {
	lines := make([]editLine, 0)
	start := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '\r':
			lines = append(lines, editLine{text: s[start:i], start: start, end: i})
			i++
			if i < len(s) && s[i] == '\n' {
				i++
			}
			start = i
		case '\n':
			lines = append(lines, editLine{text: s[start:i], start: start, end: i})
			i++
			start = i
		default:
			i++
		}
	}
	if start < len(s) {
		lines = append(lines, editLine{text: s[start:], start: start, end: len(s)})
	}
	return lines
}
