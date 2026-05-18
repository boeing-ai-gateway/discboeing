package toolrenderers

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type EditView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type EditInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

type EditOutput struct {
	Success      *bool  `json:"success"`
	Replacements *int   `json:"replacements"`
	Error        string `json:"error"`
}

type editDiffSegment struct {
	Text    string
	Changed bool
}

type editDiffRow struct {
	OldLineNumber *int
	NewLineNumber *int
	Kind          string
	Segments      []editDiffSegment
}

func parseEditInput(input string) (EditInput, bool) {
	if strings.TrimSpace(input) == "" {
		return EditInput{}, false
	}
	var parsed EditInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return EditInput{}, false
	}
	return parsed, parsed.FilePath != ""
}

func parseEditOutput(output string) (EditOutput, bool) {
	if strings.TrimSpace(output) == "" {
		return EditOutput{}, false
	}
	var parsed EditOutput
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return EditOutput{}, false
	}
	return parsed, true
}

func editHeader(input EditInput, inputOK bool, state string) string {
	if inputOK && input.FilePath != "" {
		return filepath.Base(input.FilePath)
	}
	if isStreamingState(state) {
		return "Loading edit details..."
	}
	return "Edit file"
}

func buildEditDiffRows(oldContent string, newContent string) []editDiffRow {
	oldLines := editLines(oldContent)
	newLines := editLines(newContent)
	matrix := editLCSMatrix(oldLines, newLines)
	operations := make([]editDiffRow, 0, len(oldLines)+len(newLines))
	oldIndex := len(oldLines)
	newIndex := len(newLines)

	for oldIndex > 0 && newIndex > 0 {
		if oldLines[oldIndex-1] == newLines[newIndex-1] {
			operations = append(operations, editDiffRow{Kind: "context", OldLineNumber: new(oldIndex), NewLineNumber: new(newIndex), Segments: []editDiffSegment{{Text: oldLines[oldIndex-1]}}})
			oldIndex--
			newIndex--
			continue
		}
		if matrix[oldIndex-1][newIndex] >= matrix[oldIndex][newIndex-1] {
			operations = append(operations, editDiffRow{Kind: "remove", OldLineNumber: new(oldIndex), Segments: []editDiffSegment{{Text: oldLines[oldIndex-1], Changed: true}}})
			oldIndex--
			continue
		}
		operations = append(operations, editDiffRow{Kind: "add", NewLineNumber: new(newIndex), Segments: []editDiffSegment{{Text: newLines[newIndex-1], Changed: true}}})
		newIndex--
	}
	for oldIndex > 0 {
		operations = append(operations, editDiffRow{Kind: "remove", OldLineNumber: new(oldIndex), Segments: []editDiffSegment{{Text: oldLines[oldIndex-1], Changed: true}}})
		oldIndex--
	}
	for newIndex > 0 {
		operations = append(operations, editDiffRow{Kind: "add", NewLineNumber: new(newIndex), Segments: []editDiffSegment{{Text: newLines[newIndex-1], Changed: true}}})
		newIndex--
	}
	reverseEditRows(operations)
	return pairEditRows(operations)
}

func pairEditRows(operations []editDiffRow) []editDiffRow {
	rows := make([]editDiffRow, 0, len(operations))
	for index := 0; index < len(operations); {
		operation := operations[index]
		if operation.Kind == "context" {
			rows = append(rows, operation)
			index++
			continue
		}

		var removed []editDiffRow
		var added []editDiffRow
		for index < len(operations) && operations[index].Kind != "context" {
			if operations[index].Kind == "remove" {
				removed = append(removed, operations[index])
			} else {
				added = append(added, operations[index])
			}
			index++
		}
		pairCount := max(len(removed), len(added))
		for pairIndex := range pairCount {
			var leftSegments, rightSegments []editDiffSegment
			if pairIndex < len(removed) && pairIndex < len(added) {
				leftSegments, rightSegments = editChangedLineSegments(removed[pairIndex].Segments[0].Text, added[pairIndex].Segments[0].Text)
			}
			if pairIndex < len(removed) {
				row := removed[pairIndex]
				if leftSegments != nil {
					row.Segments = leftSegments
				}
				rows = append(rows, row)
			}
			if pairIndex < len(added) {
				row := added[pairIndex]
				if rightSegments != nil {
					row.Segments = rightSegments
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func editLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func editLCSMatrix(left []string, right []string) [][]int {
	matrix := make([][]int, len(left)+1)
	for index := range matrix {
		matrix[index] = make([]int, len(right)+1)
	}
	for leftIndex := 1; leftIndex <= len(left); leftIndex++ {
		for rightIndex := 1; rightIndex <= len(right); rightIndex++ {
			if left[leftIndex-1] == right[rightIndex-1] {
				matrix[leftIndex][rightIndex] = matrix[leftIndex-1][rightIndex-1] + 1
			} else {
				matrix[leftIndex][rightIndex] = max(matrix[leftIndex-1][rightIndex], matrix[leftIndex][rightIndex-1])
			}
		}
	}
	return matrix
}

func editChangedLineSegments(left string, right string) ([]editDiffSegment, []editDiffSegment) {
	prefixLength := commonPrefixLength(left, right)
	suffixLength := commonSuffixLength(left, right, prefixLength)
	return editSegments(left, prefixLength, suffixLength), editSegments(right, prefixLength, suffixLength)
}

func editSegments(value string, prefixLength int, suffixLength int) []editDiffSegment {
	suffixStart := len(value)
	if suffixLength > 0 {
		suffixStart = len(value) - suffixLength
	}
	segments := []editDiffSegment{}
	if prefix := value[:prefixLength]; prefix != "" {
		segments = append(segments, editDiffSegment{Text: prefix})
	}
	if changed := value[prefixLength:suffixStart]; changed != "" {
		segments = append(segments, editDiffSegment{Text: changed, Changed: true})
	}
	if suffix := value[suffixStart:]; suffix != "" {
		segments = append(segments, editDiffSegment{Text: suffix})
	}
	if len(segments) == 0 {
		segments = append(segments, editDiffSegment{})
	}
	return segments
}

func commonPrefixLength(left string, right string) int {
	limit := min(len(left), len(right))
	index := 0
	for index < limit && left[index] == right[index] {
		index++
	}
	return index
}

func commonSuffixLength(left string, right string, prefixLength int) int {
	limit := min(len(left), len(right)) - prefixLength
	index := 0
	for index < limit && left[len(left)-index-1] == right[len(right)-index-1] {
		index++
	}
	return index
}

func reverseEditRows(rows []editDiffRow) {
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
}

func changedEditRowCount(rows []editDiffRow) int {
	count := 0
	for _, row := range rows {
		if row.Kind != "context" {
			count++
		}
	}
	return count
}

func editLineCount(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(normalizeNewlines(value), "\n") + 1
}

func editRowClass(kind string) string {
	switch kind {
	case "add":
		return "bg-green-500/5"
	case "remove":
		return "bg-red-500/5"
	default:
		return "bg-background/70"
	}
}

func editMarker(kind string) string {
	switch kind {
	case "add":
		return "+"
	case "remove":
		return "-"
	default:
		return " "
	}
}

func editMarkerClass(kind string) string {
	switch kind {
	case "add":
		return "text-green-700 dark:text-green-400"
	case "remove":
		return "text-red-700 dark:text-red-400"
	default:
		return "text-muted-foreground"
	}
}

func editSegmentClass(kind string, changed bool) string {
	if !changed {
		return ""
	}
	if kind == "add" {
		return "rounded-sm bg-green-500/15 text-green-950 dark:text-green-50 px-0.5"
	}
	if kind == "remove" {
		return "rounded-sm bg-red-500/15 text-red-950 dark:text-red-50 px-0.5"
	}
	return ""
}

func editLineNumber(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(*value)
}
