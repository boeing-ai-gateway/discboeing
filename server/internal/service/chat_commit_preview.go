package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
)

type CommitPullPreviewResponse struct {
	CommitCount int                       `json:"commitCount"`
	HeadCommit  string                    `json:"headCommit"`
	RawPatch    string                    `json:"rawPatch"`
	Stats       CommitPullPreviewStats    `json:"stats"`
	Commits     []CommitPullPreviewCommit `json:"commits"`
}

type CommitPullPreviewStats struct {
	FilesChanged int `json:"filesChanged"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	LineCount    int `json:"lineCount"`
}

type CommitPullPreviewCommit struct {
	Hash        string                  `json:"hash"`
	Subject     string                  `json:"subject"`
	Body        string                  `json:"body,omitempty"`
	AuthorName  string                  `json:"authorName,omitempty"`
	AuthorEmail string                  `json:"authorEmail,omitempty"`
	Date        string                  `json:"date,omitempty"`
	SignedOffBy []string                `json:"signedOffBy,omitempty"`
	Stats       CommitPullPreviewStats  `json:"stats"`
	Files       []CommitPullPreviewFile `json:"files"`
}

type CommitPullPreviewFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"oldPath,omitempty"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	LineCount int    `json:"lineCount"`
	Binary    bool   `json:"binary"`
	Patch     string `json:"patch,omitempty"`
}

type requestCommitPullQuestionMetadata struct {
	Directory  string `json:"directory"`
	CommitHash string `json:"commitHash"`
}

var (
	requestCommitPullPreviewContext = "request_commit_pull"
	formatPatchCommitStartRE        = regexp.MustCompile(`(?m)^From ([0-9a-f]{7,40}) `)
	formatPatchSubjectRE            = regexp.MustCompile(`^\[PATCH[^\]]*\]\s*`)
	signedOffByRE                   = regexp.MustCompile(`(?mi)^Signed-off-by:\s*(.+?)\s*$`)
	diffGitHeaderRE                 = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
)

func (c *ChatService) GetRequestCommitPullPreview(ctx context.Context, projectID, sessionID, threadID, questionID string) (*CommitPullPreviewResponse, error) {
	if _, err := c.GetSession(ctx, projectID, sessionID); err != nil {
		return nil, err
	}
	if c.sandboxService == nil {
		return nil, fmt.Errorf("sandbox provider not available")
	}

	client, err := c.sandboxService.GetClient(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	question, err := client.GetQuestion(ctx, threadID, questionID)
	if err != nil {
		return nil, fmt.Errorf("get pending question: %w", err)
	}

	metadata, ok := requestCommitPullPreviewMetadata(question)
	if !ok {
		return nil, fmt.Errorf("request commit pull preview is unavailable for this question")
	}

	commitsResp, err := client.GetCommits(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("get sandbox commits: %w", err)
	}
	if err := validateRequestedCommitHash(metadata.CommitHash, commitsResp.HeadCommit); err != nil {
		return nil, err
	}

	preview, err := parseCommitPullPreview(commitsResp.Patches, commitsResp.HeadCommit)
	if err != nil {
		return nil, err
	}
	if preview.CommitCount == 0 {
		preview.CommitCount = commitsResp.CommitCount
	}
	return preview, nil
}

func requestCommitPullPreviewMetadata(question *sandboxapi.PendingQuestionResponse) (requestCommitPullQuestionMetadata, bool) {
	if question == nil || question.Status != "pending" || question.Question == nil {
		return requestCommitPullQuestionMetadata{}, false
	}
	if !strings.EqualFold(strings.TrimSpace(question.Question.Context), requestCommitPullPreviewContext) {
		return requestCommitPullQuestionMetadata{}, false
	}

	var metadata requestCommitPullQuestionMetadata
	if len(question.Question.Metadata) > 0 {
		if err := json.Unmarshal(question.Question.Metadata, &metadata); err != nil {
			return requestCommitPullQuestionMetadata{}, false
		}
	}

	return metadata, true
}

func parseCommitPullPreview(rawPatch, headCommit string) (*CommitPullPreviewResponse, error) {
	normalized := strings.ReplaceAll(rawPatch, "\r\n", "\n")
	normalized = strings.TrimSpace(normalized)
	preview := &CommitPullPreviewResponse{
		HeadCommit: headCommit,
		RawPatch:   normalized,
		Commits:    []CommitPullPreviewCommit{},
	}
	if normalized == "" {
		return preview, nil
	}

	parts := splitFormatPatchCommits(normalized)
	preview.Commits = make([]CommitPullPreviewCommit, 0, len(parts))
	for _, part := range parts {
		commit, err := parseFormatPatchCommit(part)
		if err != nil {
			return nil, fmt.Errorf("parse commit preview: %w", err)
		}
		preview.Commits = append(preview.Commits, commit)
		preview.Stats.FilesChanged += commit.Stats.FilesChanged
		preview.Stats.Additions += commit.Stats.Additions
		preview.Stats.Deletions += commit.Stats.Deletions
		preview.Stats.LineCount += commit.Stats.LineCount
	}
	preview.CommitCount = len(preview.Commits)
	return preview, nil
}

func splitFormatPatchCommits(rawPatch string) []string {
	matches := formatPatchCommitStartRE.FindAllStringSubmatchIndex(rawPatch, -1)
	if len(matches) == 0 {
		return []string{rawPatch}
	}

	parts := make([]string, 0, len(matches))
	for index, match := range matches {
		start := match[0]
		end := len(rawPatch)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		part := strings.TrimSpace(rawPatch[start:end])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func parseFormatPatchCommit(rawCommit string) (CommitPullPreviewCommit, error) {
	lines := strings.Split(rawCommit, "\n")
	if len(lines) == 0 {
		return CommitPullPreviewCommit{}, fmt.Errorf("empty patch commit")
	}

	match := formatPatchCommitStartRE.FindStringSubmatch(lines[0])
	if len(match) < 2 {
		return CommitPullPreviewCommit{}, fmt.Errorf("missing format-patch commit header")
	}

	headers := map[string]string{}
	index := 1
	for index < len(lines) {
		line := lines[index]
		index++
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		headers[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}

	messageStart := index
	diffStart := len(lines)
	messageEnd := len(lines)
	for i := messageStart; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "diff --git ") {
			diffStart = i
			if messageEnd == len(lines) {
				messageEnd = i
			}
			break
		}
		if line == "---" && messageEnd == len(lines) {
			messageEnd = i
		}
	}
	if messageEnd == len(lines) {
		messageEnd = diffStart
	}

	body := strings.TrimSpace(strings.Join(lines[messageStart:messageEnd], "\n"))
	files := parseFormatPatchFiles(lines[diffStart:])
	stats := CommitPullPreviewStats{FilesChanged: len(files)}
	for _, file := range files {
		stats.Additions += file.Additions
		stats.Deletions += file.Deletions
		stats.LineCount += file.LineCount
	}

	authorName, authorEmail := parsePatchAuthor(headers["From"])
	return CommitPullPreviewCommit{
		Hash:        strings.TrimSpace(match[1]),
		Subject:     strings.TrimSpace(formatPatchSubjectRE.ReplaceAllString(headers["Subject"], "")),
		Body:        body,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
		Date:        strings.TrimSpace(headers["Date"]),
		SignedOffBy: extractSignedOffBy(body),
		Stats:       stats,
		Files:       files,
	}, nil
}

func parsePatchAuthor(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return value, ""
	}
	return addr.Name, addr.Address
}

func extractSignedOffBy(body string) []string {
	matches := signedOffByRE.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func parseFormatPatchFiles(lines []string) []CommitPullPreviewFile {
	if len(lines) == 0 {
		return nil
	}

	var files []CommitPullPreviewFile
	for index := 0; index < len(lines); {
		line := lines[index]
		if !strings.HasPrefix(line, "diff --git ") {
			index++
			continue
		}

		next := index + 1
		for next < len(lines) && !strings.HasPrefix(lines[next], "diff --git ") {
			next++
		}
		files = append(files, parseFormatPatchFile(lines[index:next]))
		index = next
	}
	return files
}

func parseFormatPatchFile(lines []string) CommitPullPreviewFile {
	file := CommitPullPreviewFile{Status: "modified"}
	if len(lines) == 0 {
		return file
	}

	if match := diffGitHeaderRE.FindStringSubmatch(lines[0]); len(match) == 3 {
		file.OldPath = sanitizePatchPath(match[1])
		file.Path = sanitizePatchPath(match[2])
	}

	inHunk := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "new file mode "):
			file.Status = "added"
		case strings.HasPrefix(line, "deleted file mode "):
			file.Status = "deleted"
		case strings.HasPrefix(line, "rename from "):
			file.Status = "renamed"
			file.OldPath = strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
		case strings.HasPrefix(line, "rename to "):
			file.Status = "renamed"
			file.Path = strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "Binary files "):
			file.Binary = true
		case line == "GIT binary patch":
			file.Binary = true
		case strings.HasPrefix(line, "--- "):
			oldPath := sanitizePatchPath(strings.TrimSpace(strings.TrimPrefix(line, "--- ")))
			if oldPath != "" {
				file.OldPath = oldPath
			}
		case strings.HasPrefix(line, "+++ "):
			newPath := sanitizePatchPath(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
			if newPath != "" {
				file.Path = newPath
			}
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			file.Additions++
		case inHunk && strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			file.Deletions++
		}
	}

	if file.Path == "" {
		file.Path = file.OldPath
	}
	if file.Status == "deleted" && file.OldPath != "" {
		file.Path = file.OldPath
	}
	if file.Status != "renamed" && file.Path == file.OldPath {
		file.OldPath = ""
	}

	file.LineCount = file.Additions + file.Deletions
	file.Patch = strings.TrimSpace(strings.Join(lines, "\n"))
	return file
}

func sanitizePatchPath(value string) string {
	value = strings.TrimSpace(value)
	switch {
	case value == "", value == "/dev/null":
		return ""
	case strings.HasPrefix(value, "a/"), strings.HasPrefix(value, "b/"):
		return value[2:]
	default:
		return value
	}
}
