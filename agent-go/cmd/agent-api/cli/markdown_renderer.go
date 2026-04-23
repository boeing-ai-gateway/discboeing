package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// markdownRenderer provides a tiny streaming markdown formatter for text chunks.
// It intentionally supports only basic formatting, buffers fenced code blocks
// until they are complete, and renders pipe tables once the full table block is
// available.
type markdownRenderer struct {
	out          io.Writer
	enableFormat bool
	enableColor  bool
	atLineStart  bool

	pending string

	inFence        bool
	fenceDelimiter string
	fenceBuf       strings.Builder

	pendingTableHeader string
	tableLines         []string
}

func newMarkdownRenderer(out io.Writer, enableFormat, enableColor bool) *markdownRenderer {
	if !enableFormat {
		enableColor = false
	}
	return &markdownRenderer{
		out:          out,
		enableFormat: enableFormat,
		enableColor:  enableColor,
		atLineStart:  true,
	}
}

func (r *markdownRenderer) WriteText(delta string) {
	if delta == "" {
		return
	}

	r.pending += delta
	for {
		idx := strings.IndexByte(r.pending, '\n')
		if idx < 0 {
			break
		}
		line := r.pending[:idx+1]
		r.pending = r.pending[idx+1:]
		r.consumeCompleteLine(line)
	}

	r.flushTailIfSafe()
}

// FlushForBoundary flushes pending non-block text before printing non-text chunks.
// If currently inside a fenced code block, data remains buffered until the fence
// closes or Finish() is called.
func (r *markdownRenderer) FlushForBoundary() {
	if r.inFence {
		return
	}
	if r.pendingTableHeader != "" {
		r.emitFormattedLine(r.pendingTableHeader)
		r.pendingTableHeader = ""
	}
	if len(r.tableLines) > 0 {
		r.emitTable()
	}
	if r.pending == "" {
		return
	}
	r.emitInlineSegment(r.pending)
	r.pending = ""
}

// Finish flushes all remaining buffered content at end-of-turn.
func (r *markdownRenderer) Finish() {
	if r.pending != "" {
		if r.inFence {
			r.fenceBuf.WriteString(r.pending)
		} else {
			r.emitInlineSegment(r.pending)
		}
		r.pending = ""
	}

	if r.pendingTableHeader != "" {
		r.emitFormattedLine(r.pendingTableHeader)
		r.pendingTableHeader = ""
	}
	if len(r.tableLines) > 0 {
		r.emitTable()
	}

	if r.inFence {
		r.emitCodeBlock(r.fenceBuf.String())
		r.fenceBuf.Reset()
		r.inFence = false
		r.fenceDelimiter = ""
	}
}

func (r *markdownRenderer) AtLineStart() bool {
	if r == nil {
		return true
	}
	return r.atLineStart
}

func (r *markdownRenderer) consumeCompleteLine(line string) {
	for {
		if r.inFence {
			r.fenceBuf.WriteString(line)
			if r.isFenceCloseLine(line) {
				r.emitCodeBlock(r.fenceBuf.String())
				r.fenceBuf.Reset()
				r.inFence = false
				r.fenceDelimiter = ""
			}
			return
		}

		if len(r.tableLines) > 0 {
			content, _ := splitLineEnding(line)
			if _, ok := parseTableRow(content); ok {
				r.tableLines = append(r.tableLines, line)
				return
			}
			r.emitTable()
			continue
		}

		if r.pendingTableHeader != "" {
			content, _ := splitLineEnding(line)
			if _, ok := parseTableSeparator(content); ok {
				r.tableLines = []string{r.pendingTableHeader, line}
				r.pendingTableHeader = ""
				return
			}
			r.emitFormattedLine(r.pendingTableHeader)
			r.pendingTableHeader = ""
			continue
		}

		if delim, ok := detectFenceOpenLine(line); ok {
			r.inFence = true
			r.fenceDelimiter = delim
			r.fenceBuf.Reset()
			r.fenceBuf.WriteString(line)
			return
		}

		content, _ := splitLineEnding(line)
		if _, ok := parseTableRow(content); ok {
			r.pendingTableHeader = line
			return
		}

		r.emitFormattedLine(line)
		return
	}
}

func (r *markdownRenderer) flushTailIfSafe() {
	if r.inFence || r.pending == "" {
		return
	}
	if !r.enableFormat {
		r.write(r.pending)
		r.pending = ""
		return
	}
	if tailNeedsMoreInput(r.pending) {
		return
	}
	r.emitInlineSegment(r.pending)
	r.pending = ""
}

func tailNeedsMoreInput(s string) bool {
	trim := strings.TrimLeft(s, " \t")
	if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
		return true
	}
	if strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, ">") {
		return true
	}
	if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") || strings.HasPrefix(trim, "+ ") {
		return true
	}
	if _, _, ok := parseOrderedList(trim); ok {
		return true
	}
	if _, ok := parseTableRow(trim); ok {
		return true
	}
	return strings.ContainsAny(s, "`*")
}

func detectFenceOpenLine(line string) (string, bool) {
	trim := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(trim, "```"):
		return "```", true
	case strings.HasPrefix(trim, "~~~"):
		return "~~~", true
	default:
		return "", false
	}
}

func (r *markdownRenderer) isFenceCloseLine(line string) bool {
	trim := strings.TrimSpace(line)
	return strings.HasPrefix(trim, r.fenceDelimiter)
}

func (r *markdownRenderer) emitFormattedLine(line string) {
	if !r.enableFormat {
		r.write(line)
		return
	}

	content, suffix := splitLineEnding(line)
	r.write(r.formatBlockLine(content))
	r.write(suffix)
}

func (r *markdownRenderer) emitInlineSegment(seg string) {
	if !r.enableFormat {
		r.write(seg)
		return
	}
	r.write(r.formatInline(seg))
}

func (r *markdownRenderer) emitCodeBlock(block string) {
	if !r.enableFormat || !r.enableColor {
		r.write(block)
		return
	}
	r.write(r.style(block, "38;5;245"))
}

func (r *markdownRenderer) emitTable() {
	if len(r.tableLines) < 2 {
		for _, line := range r.tableLines {
			r.emitFormattedLine(line)
		}
		r.tableLines = nil
		return
	}

	headerCells, ok := parseTableRow(splitLineContent(r.tableLines[0]))
	if !ok {
		for _, line := range r.tableLines {
			r.emitFormattedLine(line)
		}
		r.tableLines = nil
		return
	}
	alignments, ok := parseTableSeparator(splitLineContent(r.tableLines[1]))
	if !ok {
		for _, line := range r.tableLines {
			r.emitFormattedLine(line)
		}
		r.tableLines = nil
		return
	}

	rows := make([][]string, 0, len(r.tableLines)-2)
	for _, line := range r.tableLines[2:] {
		cells, ok := parseTableRow(splitLineContent(line))
		if !ok {
			continue
		}
		rows = append(rows, cells)
	}

	colCount := max(len(alignments), len(headerCells))
	for _, row := range rows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	if colCount == 0 {
		r.tableLines = nil
		return
	}

	headerCells = normalizeTableCells(headerCells, colCount)
	alignments = normalizeAlignments(alignments, colCount)
	for i := range rows {
		rows[i] = normalizeTableCells(rows[i], colCount)
	}

	widths := make([]int, colCount)
	for i, cell := range headerCells {
		widths[i] = displayWidth(cell)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	_, ending := splitLineEnding(r.tableLines[0])
	if ending == "" {
		ending = "\n"
	}

	r.write(r.renderTableRow(headerCells, widths, alignments, true))
	r.write(ending)
	r.write(r.renderTableSeparator(widths, alignments))
	r.write(ending)
	for _, row := range rows {
		r.write(r.renderTableRow(row, widths, alignments, false))
		r.write(ending)
	}

	r.tableLines = nil
}

func splitLineEnding(line string) (content string, ending string) {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return strings.TrimSuffix(line, "\r\n"), "\r\n"
	case strings.HasSuffix(line, "\n"):
		return strings.TrimSuffix(line, "\n"), "\n"
	default:
		return line, ""
	}
}

func splitLineContent(line string) string {
	content, _ := splitLineEnding(line)
	return content
}

func (r *markdownRenderer) formatBlockLine(line string) string {
	indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
	indent := line[:indentLen]
	trim := line[indentLen:]

	if level, text, ok := parseHeading(trim); ok {
		heading := strings.Repeat("#", level) + " " + text
		return indent + r.style(heading, "1;36")
	}
	if text, ok := parseBlockquote(trim); ok {
		if text == "" {
			return indent + r.style(">", "36")
		}
		return indent + r.style(">", "36") + " " + r.formatInline(text)
	}
	if marker, text, ok := parseUnorderedList(trim); ok {
		return indent + r.style(marker, "33") + " " + r.formatInline(text)
	}
	if marker, text, ok := parseOrderedList(trim); ok {
		return indent + r.style(marker, "33") + " " + r.formatInline(text)
	}

	return indent + r.formatInline(trim)
}

func parseHeading(s string) (int, string, bool) {
	level := 0
	for level < len(s) && s[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if level >= len(s) || s[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(s[level+1:]), true
}

func parseBlockquote(s string) (string, bool) {
	if !strings.HasPrefix(s, ">") {
		return "", false
	}
	text := strings.TrimPrefix(s, ">")
	text = strings.TrimPrefix(text, " ")
	return text, true
}

func parseUnorderedList(s string) (marker, text string, ok bool) {
	if len(s) < 2 {
		return "", "", false
	}
	if (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return s[:1], strings.TrimLeft(s[2:], " "), true
	}
	return "", "", false
}

func parseOrderedList(s string) (marker, text string, ok bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(s) || s[i] != '.' || s[i+1] != ' ' {
		return "", "", false
	}
	return s[:i+1], strings.TrimLeft(s[i+2:], " "), true
}

type tableAlignment int

const (
	tableAlignLeft tableAlignment = iota
	tableAlignCenter
	tableAlignRight
)

func parseTableRow(line string) ([]string, bool) {
	trim := strings.TrimSpace(line)
	if trim == "" || !strings.Contains(trim, "|") {
		return nil, false
	}

	parts := splitMarkdownTableCells(trim)
	if len(parts) < 2 {
		return nil, false
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, true
}

func parseTableSeparator(line string) ([]tableAlignment, bool) {
	cells, ok := parseTableRow(line)
	if !ok {
		return nil, false
	}

	alignments := make([]tableAlignment, len(cells))
	for i, cell := range cells {
		align, ok := parseTableSeparatorCell(cell)
		if !ok {
			return nil, false
		}
		alignments[i] = align
	}
	return alignments, true
}

func splitMarkdownTableCells(line string) []string {
	trim := strings.TrimSpace(line)
	trim = strings.TrimPrefix(trim, "|")
	trim = strings.TrimSuffix(trim, "|")

	var cells []string
	var cell strings.Builder
	escaped := false
	for _, ch := range trim {
		switch {
		case escaped:
			cell.WriteRune(ch)
			escaped = false
		case ch == '\\':
			escaped = true
			cell.WriteRune(ch)
		case ch == '|':
			cells = append(cells, cell.String())
			cell.Reset()
		default:
			cell.WriteRune(ch)
		}
	}
	cells = append(cells, cell.String())
	return cells
}

func parseTableSeparatorCell(cell string) (tableAlignment, bool) {
	trim := strings.TrimSpace(cell)
	if len(trim) < 3 {
		return tableAlignLeft, false
	}

	left := strings.HasPrefix(trim, ":")
	right := strings.HasSuffix(trim, ":")
	core := trim
	if left {
		core = core[1:]
	}
	if right {
		core = core[:len(core)-1]
	}
	if len(core) < 3 {
		return tableAlignLeft, false
	}
	for _, ch := range core {
		if ch != '-' {
			return tableAlignLeft, false
		}
	}

	switch {
	case left && right:
		return tableAlignCenter, true
	case right:
		return tableAlignRight, true
	default:
		return tableAlignLeft, true
	}
}

func normalizeTableCells(cells []string, width int) []string {
	out := make([]string, width)
	copy(out, cells)
	return out
}

func normalizeAlignments(alignments []tableAlignment, width int) []tableAlignment {
	out := make([]tableAlignment, width)
	copy(out, alignments)
	return out
}

func (r *markdownRenderer) renderTableRow(cells []string, widths []int, alignments []tableAlignment, header bool) string {
	var out strings.Builder
	out.WriteString("|")
	for i, cell := range cells {
		raw := strings.TrimSpace(cell)
		formatted := r.formatInline(raw)
		if header && r.enableColor {
			formatted = r.style(formatted, "1")
		}
		out.WriteString(" ")
		out.WriteString(padTableCell(formatted, raw, widths[i], alignments[i]))
		out.WriteString(" |")
	}
	return out.String()
}

func (r *markdownRenderer) renderTableSeparator(widths []int, alignments []tableAlignment) string {
	var out strings.Builder
	out.WriteString("|")
	for i, width := range widths {
		segment := strings.Repeat("-", maxInt(width, 3))
		switch alignments[i] {
		case tableAlignCenter:
			segment = ":" + segment + ":"
		case tableAlignRight:
			segment = strings.Repeat("-", maxInt(width-1, 2)) + ":"
		default:
			segment = strings.Repeat("-", maxInt(width, 3))
		}
		if alignments[i] == tableAlignLeft {
			out.WriteString(" ")
			out.WriteString(segment)
			out.WriteString(" |")
			continue
		}
		out.WriteString(" ")
		out.WriteString(segment)
		out.WriteString(" |")
	}
	return out.String()
}

func padTableCell(formatted, raw string, width int, align tableAlignment) string {
	padding := width - displayWidth(raw)
	if padding <= 0 {
		return formatted
	}

	switch align {
	case tableAlignRight:
		return strings.Repeat(" ", padding) + formatted
	case tableAlignCenter:
		left := padding / 2
		right := padding - left
		return strings.Repeat(" ", left) + formatted + strings.Repeat(" ", right)
	default:
		return formatted + strings.Repeat(" ", padding)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

func (r *markdownRenderer) formatInline(text string) string {
	if !r.enableColor {
		return text
	}

	var out strings.Builder
	remaining := text

	for {
		start := strings.Index(remaining, "`")
		if start < 0 {
			out.WriteString(r.formatEmphasis(remaining))
			break
		}
		end := strings.Index(remaining[start+1:], "`")
		if end < 0 {
			out.WriteString(r.formatEmphasis(remaining))
			break
		}

		out.WriteString(r.formatEmphasis(remaining[:start]))
		code := remaining[start+1 : start+1+end]
		out.WriteString(r.style(code, "38;5;214"))
		remaining = remaining[start+1+end+1:]
	}

	return out.String()
}

func (r *markdownRenderer) formatEmphasis(text string) string {
	text = applyPairedStyle(text, "**", func(inner string) string {
		return r.style(inner, "1")
	})
	text = applyPairedStyle(text, "*", func(inner string) string {
		return r.style(inner, "3")
	})
	return text
}

func applyPairedStyle(text, delim string, style func(string) string) string {
	if text == "" {
		return text
	}

	var out strings.Builder
	remaining := text

	for {
		start := strings.Index(remaining, delim)
		if start < 0 {
			out.WriteString(remaining)
			break
		}
		endOffset := strings.Index(remaining[start+len(delim):], delim)
		if endOffset < 0 {
			out.WriteString(remaining)
			break
		}

		out.WriteString(remaining[:start])
		inner := remaining[start+len(delim) : start+len(delim)+endOffset]
		out.WriteString(style(inner))
		remaining = remaining[start+len(delim)+endOffset+len(delim):]
	}

	return out.String()
}

func (r *markdownRenderer) style(text, code string) string {
	if !r.enableColor || text == "" {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

func (r *markdownRenderer) write(text string) {
	if text == "" {
		return
	}
	_, _ = fmt.Fprint(r.out, text)
	r.atLineStart = strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\r")
}
