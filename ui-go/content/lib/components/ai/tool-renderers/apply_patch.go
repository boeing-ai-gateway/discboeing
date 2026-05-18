package toolrenderers

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ApplyPatchView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type applyPatchLine struct {
	Marker  string
	Content string
}

type applyPatchChunk struct {
	Context     string
	Lines       []applyPatchLine
	IsEndOfFile bool
}

type applyPatchOperation struct {
	Kind      string
	Path      string
	MovePath  string
	AddLines  []string
	Chunks    []applyPatchChunk
	Additions int
	Removals  int
}

type applyPatchStats struct {
	Files     int
	Additions int
	Removals  int
}

type applyPatchParseResult struct {
	Raw        string
	Operations []applyPatchOperation
	Stats      applyPatchStats
	Incomplete bool
	Error      string
}

type applyPatchOutputEntry struct {
	Marker string
	Path   string
}

type applyPatchOutputResult struct {
	Raw     string
	Entries []applyPatchOutputEntry
}

const (
	beginPatchMarker  = "*** Begin Patch"
	endPatchMarker    = "*** End Patch"
	addFileMarker     = "*** Add File: "
	deleteFileMarker  = "*** Delete File: "
	updateFileMarker  = "*** Update File: "
	moveToMarker      = "*** Move to: "
	endOfFileMarker   = "*** End of File"
	patchOutputPrefix = "Success. Updated the following files:"
)

func classNames(parts ...string) string {
	classes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			classes = append(classes, trimmed)
		}
	}
	return strings.Join(classes, " ")
}

func isStreamingState(state string) bool {
	return state == "input-streaming" || state == "input-available"
}

func applyPatchHeadline(view ApplyPatchView) string {
	if title := summarizeApplyPatchTitle(view.Input); title != "" {
		return title
	}
	if isStreamingState(view.State) {
		return "Loading patch details..."
	}
	return "Apply patch"
}

func parseApplyPatch(rawPatch string) applyPatchParseResult {
	raw := normalizeNewlines(rawPatch)
	if strings.TrimSpace(raw) == "" {
		return emptyApplyPatchParseResult(raw, "Patch input is empty.")
	}

	lines := strings.Split(raw, "\n")
	beginIndex := firstNonEmptyLine(lines)
	if beginIndex < 0 || strings.TrimSpace(lines[beginIndex]) != beginPatchMarker {
		return emptyApplyPatchParseResult(raw, fmt.Sprintf("Patch must start with '%s'.", beginPatchMarker))
	}

	endIndex := patchEndIndex(lines, beginIndex+1)
	incomplete := endIndex < 0
	bodyEnd := endIndex
	if incomplete {
		bodyEnd = len(lines)
	}

	result := applyPatchParseResult{Raw: raw, Incomplete: incomplete}
	for cursor := beginIndex + 1; cursor < bodyEnd; {
		if strings.TrimSpace(lines[cursor]) == "" {
			cursor++
			continue
		}

		operation, nextCursor, parseError := parsePatchOperation(lines, cursor, bodyEnd)
		if parseError != "" {
			result.Error = parseError
			break
		}
		if nextCursor <= cursor {
			break
		}
		result.Operations = append(result.Operations, operation)
		cursor = nextCursor
	}

	for _, operation := range result.Operations {
		result.Stats.Files++
		result.Stats.Additions += operation.Additions
		result.Stats.Removals += operation.Removals
	}
	return result
}

func emptyApplyPatchParseResult(raw string, err string) applyPatchParseResult {
	return applyPatchParseResult{Raw: raw, Error: err}
}

func parseApplyPatchOutput(output string) applyPatchOutputResult {
	raw := normalizeNewlines(output)
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, patchOutputPrefix) {
		return applyPatchOutputResult{Raw: raw}
	}

	var entries []applyPatchOutputEntry
	for line := range strings.SplitSeq(strings.TrimSpace(strings.TrimPrefix(trimmed, patchOutputPrefix)), "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 2 {
			continue
		}
		marker := line[:1]
		if marker != "A" && marker != "M" && marker != "D" {
			continue
		}
		path := strings.TrimSpace(line[1:])
		if path != "" {
			entries = append(entries, applyPatchOutputEntry{Marker: marker, Path: path})
		}
	}
	return applyPatchOutputResult{Raw: raw, Entries: entries}
}

func summarizeApplyPatchTitle(rawPatch string) string {
	rawPatch = normalizeNewlines(rawPatch)
	if strings.TrimSpace(rawPatch) == "" {
		return ""
	}

	var firstPath, pendingUpdatePath string
	operationCount := 0
	for line := range strings.SplitSeq(rawPatch, "\n") {
		switch {
		case strings.HasPrefix(line, addFileMarker):
			operationCount++
			if path := strings.TrimSpace(strings.TrimPrefix(line, addFileMarker)); firstPath == "" && path != "" {
				firstPath = path
			}
			pendingUpdatePath = ""
		case strings.HasPrefix(line, deleteFileMarker):
			operationCount++
			if path := strings.TrimSpace(strings.TrimPrefix(line, deleteFileMarker)); firstPath == "" && path != "" {
				firstPath = path
			}
			pendingUpdatePath = ""
		case strings.HasPrefix(line, updateFileMarker):
			operationCount++
			path := strings.TrimSpace(strings.TrimPrefix(line, updateFileMarker))
			if firstPath == "" && path != "" {
				firstPath = path
			}
			pendingUpdatePath = path
		case strings.HasPrefix(line, moveToMarker) && pendingUpdatePath != "":
			movePath := strings.TrimSpace(strings.TrimPrefix(line, moveToMarker))
			if movePath != "" && firstPath == pendingUpdatePath {
				firstPath = movePath
			}
			pendingUpdatePath = ""
		}
	}

	if firstPath == "" || operationCount == 0 {
		return ""
	}
	name := filepath.Base(firstPath)
	if operationCount == 1 {
		return name
	}
	return fmt.Sprintf("%s (+%d)", name, operationCount-1)
}

func parsePatchOperation(lines []string, cursor int, bodyEnd int) (applyPatchOperation, int, string) {
	header := strings.TrimSpace(lines[cursor])
	switch {
	case strings.HasPrefix(header, addFileMarker):
		operation := applyPatchOperation{Kind: "add", Path: strings.TrimSpace(strings.TrimPrefix(header, addFileMarker))}
		nextCursor := cursor + 1
		for ; nextCursor < bodyEnd; nextCursor++ {
			if !strings.HasPrefix(lines[nextCursor], "+") {
				break
			}
			operation.AddLines = append(operation.AddLines, strings.TrimPrefix(lines[nextCursor], "+"))
		}
		operation.Additions = len(operation.AddLines)
		return operation, nextCursor, ""
	case strings.HasPrefix(header, deleteFileMarker):
		return applyPatchOperation{Kind: "delete", Path: strings.TrimSpace(strings.TrimPrefix(header, deleteFileMarker))}, cursor + 1, ""
	case strings.HasPrefix(header, updateFileMarker):
		operation := applyPatchOperation{Kind: "update", Path: strings.TrimSpace(strings.TrimPrefix(header, updateFileMarker))}
		nextCursor := cursor + 1
		if nextCursor < bodyEnd && strings.HasPrefix(strings.TrimSpace(lines[nextCursor]), moveToMarker) {
			operation.MovePath = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[nextCursor]), moveToMarker))
			nextCursor++
		}
		for nextCursor < bodyEnd {
			trimmed := strings.TrimSpace(lines[nextCursor])
			if trimmed == "" {
				nextCursor++
				continue
			}
			if strings.HasPrefix(trimmed, "*** ") {
				break
			}
			chunk, cursorAfterChunk, parseError := parsePatchChunk(lines, nextCursor, bodyEnd, len(operation.Chunks) == 0)
			if parseError != "" {
				return operation, cursorAfterChunk, parseError
			}
			operation.Chunks = append(operation.Chunks, chunk)
			nextCursor = cursorAfterChunk
		}
		if len(operation.Chunks) == 0 {
			return operation, nextCursor, fmt.Sprintf("Update file hunk for '%s' is empty.", operation.Path)
		}
		for _, chunk := range operation.Chunks {
			for _, line := range chunk.Lines {
				switch line.Marker {
				case "+":
					operation.Additions++
				case "-":
					operation.Removals++
				}
			}
		}
		return operation, nextCursor, ""
	default:
		return applyPatchOperation{}, cursor, fmt.Sprintf("Unexpected patch header: '%s'.", header)
	}
}

func parsePatchChunk(lines []string, cursor int, bodyEnd int, allowMissingContext bool) (applyPatchChunk, int, string) {
	nextCursor := cursor
	chunk := applyPatchChunk{}
	firstTrimmed := strings.TrimSpace(lines[nextCursor])
	if firstTrimmed == "@@" {
		nextCursor++
	} else if context, ok := strings.CutPrefix(firstTrimmed, "@@ "); ok {
		chunk.Context = context
		nextCursor++
	} else if !allowMissingContext {
		return chunk, nextCursor, fmt.Sprintf("Expected update hunk to start with '@@', got '%s'.", lines[nextCursor])
	}

	for ; nextCursor < bodyEnd; nextCursor++ {
		line := lines[nextCursor]
		trimmed := strings.TrimSpace(line)
		if trimmed == endOfFileMarker {
			chunk.IsEndOfFile = true
			nextCursor++
			break
		}
		if len(chunk.Lines) > 0 && (trimmed == "@@" || strings.HasPrefix(trimmed, "@@ ") || strings.HasPrefix(trimmed, "*** ")) {
			break
		}
		if line == "" {
			chunk.Lines = append(chunk.Lines, applyPatchLine{Marker: " ", Content: ""})
			continue
		}
		switch line[:1] {
		case " ", "+", "-":
			chunk.Lines = append(chunk.Lines, applyPatchLine{Marker: line[:1], Content: line[1:]})
		default:
			if len(chunk.Lines) == 0 {
				return chunk, nextCursor, fmt.Sprintf("Unexpected line in update hunk: '%s'.", line)
			}
			return chunk, nextCursor, ""
		}
	}
	if len(chunk.Lines) == 0 {
		return chunk, nextCursor, "Update hunk does not contain unknown lines."
	}
	return chunk, nextCursor, ""
}

func firstNonEmptyLine(lines []string) int {
	for index, line := range lines {
		if strings.TrimSpace(line) != "" {
			return index
		}
	}
	return -1
}

func patchEndIndex(lines []string, startIndex int) int {
	for index := startIndex; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == endPatchMarker {
			return index
		}
	}
	return -1
}

func normalizeNewlines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func shortenPath(path string) string {
	if rest, ok := strings.CutPrefix(path, "/home/discobot"); ok {
		return "~" + rest
	}
	return path
}

func operationLabel(operation applyPatchOperation) string {
	switch operation.Kind {
	case "add":
		return "Add file"
	case "delete":
		if operation.MovePath != "" {
			return "Rename file"
		}
		return "Delete file"
	default:
		if operation.MovePath != "" {
			return "Move + edit"
		}
		return "Update file"
	}
}

func operationBadgeClass(operation applyPatchOperation) string {
	switch operation.Kind {
	case "add":
		return "border-emerald-200 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
	case "delete":
		return "border-red-200 bg-red-500/10 text-red-700 dark:text-red-300"
	default:
		if operation.MovePath != "" {
			return "border-sky-200 bg-sky-500/10 text-sky-700 dark:text-sky-300"
		}
		return "border-blue-200 bg-blue-500/10 text-blue-700 dark:text-blue-300"
	}
}

func markerClass(marker string) string {
	switch marker {
	case "+":
		return "text-emerald-700 dark:text-emerald-300"
	case "-":
		return "text-red-700 dark:text-red-300"
	default:
		return "text-muted-foreground"
	}
}

func rowClass(marker string) string {
	switch marker {
	case "+":
		return "bg-emerald-500/10"
	case "-":
		return "bg-red-500/10"
	default:
		return "bg-background/60"
	}
}

func resultBadgeClass(marker string) string {
	switch marker {
	case "A":
		return "border-emerald-200 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
	case "D":
		return "border-red-200 bg-red-500/10 text-red-700 dark:text-red-300"
	default:
		return "border-blue-200 bg-blue-500/10 text-blue-700 dark:text-blue-300"
	}
}

func plural(count int, singular string, pluralText string) string {
	if count == 1 {
		return singular
	}
	return pluralText
}

func lineText(value string) string {
	if value == "" {
		return " "
	}
	return value
}
