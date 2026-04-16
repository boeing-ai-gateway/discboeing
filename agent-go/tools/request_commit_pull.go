package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type requestCommitPullInput struct {
	Notes      string `json:"notes"`
	BaseCommit string `json:"baseCommit,omitempty"`
}

type requestCommitPullMetadata struct {
	Directory   string `json:"directory"`
	BaseCommit  string `json:"baseCommit,omitempty"`
	CommitHash  string `json:"commitHash"`
	CommitTitle string `json:"commitTitle,omitempty"`
	CommitBody  string `json:"commitBody,omitempty"`
}

const (
	requestCommitPullApprovalContext      = "request_commit_pull"
	requestCommitPullApprovedKey          = "__request_commit_pull_approved__"
	requestCommitPullRejectedKey          = "__request_commit_pull_rejected__"
	requestCommitPullRejectedReasonKey    = "__request_commit_pull_rejection_reason__"
	requestCommitPullSucceededKey         = "__request_commit_pull_succeeded__"
	requestCommitPullFailedKey            = "__request_commit_pull_failed__"
	requestCommitPullResultKey            = "__request_commit_pull_result__"
	requestCommitPullDefaultQuestion      = "Allow Discobot to pull the prepared sandbox commit into the host workspace?"
	requestCommitPullDefaultHeader        = "Sandbox commit pull"
	requestCommitPullApprovedResponseText = "The user approved pulling the prepared sandbox commit into the host workspace."
)

func (e *Executor) executeRequestCommitPull(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input requestCommitPullInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	question := api.AskUserQuestion{
		Question: requestCommitPullDefaultQuestion,
		Header:   requestCommitPullDefaultHeader,
		Options: []api.AskUserQuestionOption{
			{
				Label:       "Approve",
				Description: "Pull the prepared sandbox commit into the host workspace",
			},
			{
				Label:       "Reject",
				Description: "Leave the sandbox commit in the sandbox for now",
			},
		},
	}
	metadata, err := e.requestCommitPullMetadata(input.BaseCommit)
	if err != nil {
		return errResult(call, err.Error()), nil
	}
	if strings.TrimSpace(input.Notes) != "" {
		question.Notes = strings.TrimSpace(input.Notes)
	}

	payload, err := json.Marshal([]api.AskUserQuestion{question})
	if err != nil {
		return thread.ToolExecuteResult{}, fmt.Errorf("marshal request commit pull approval: %w", err)
	}
	metadataPayload, err := json.Marshal(metadata)
	if err != nil {
		return thread.ToolExecuteResult{}, fmt.Errorf("marshal request commit pull metadata: %w", err)
	}

	return thread.ToolExecuteResult{
		Approval: &thread.ApprovalRequest{
			Questions: payload,
			Metadata:  metadataPayload,
			Context:   requestCommitPullApprovalContext,
		},
	}, nil
}

func (e *Executor) requestCommitPullMetadata(requestedBaseCommit string) (*requestCommitPullMetadata, error) {
	cwd := e.getCwd()
	if _, err := gitOutput(cwd, "rev-parse", "--show-toplevel"); err != nil {
		return nil, fmt.Errorf("request commit pull requires a git repository: %w", err)
	}
	headCommit, err := gitOutput(cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("request commit pull requires at least one git commit: %w", err)
	}
	baseCommit, err := resolveRequestCommitPullBaseCommit(cwd, strings.TrimSpace(headCommit), requestedBaseCommit)
	if err != nil {
		return nil, fmt.Errorf("resolve request commit pull base commit: %w", err)
	}
	commitTitle, err := gitOutput(cwd, "log", "-1", "--format=%s", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("read request commit pull commit title: %w", err)
	}
	commitBody, err := gitOutput(cwd, "log", "-1", "--format=%b", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("read request commit pull commit body: %w", err)
	}
	absDir, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("determine request commit pull directory: %w", err)
	}
	absDir = filepath.ToSlash(strings.TrimSpace(absDir))
	return &requestCommitPullMetadata{
		Directory:   absDir,
		BaseCommit:  strings.TrimSpace(baseCommit),
		CommitHash:  strings.TrimSpace(headCommit),
		CommitTitle: strings.TrimSpace(commitTitle),
		CommitBody:  strings.TrimSpace(commitBody),
	}, nil
}

func resolveRequestCommitPullBaseCommit(cwd, headCommit, requestedBaseCommit string) (string, error) {
	if strings.TrimSpace(requestedBaseCommit) != "" {
		baseCommit, err := gitOutput(cwd, "rev-parse", strings.TrimSpace(requestedBaseCommit)+"^{commit}")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(baseCommit), nil
	}

	candidates := []string{}
	if upstream, err := gitOutput(cwd, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil && strings.TrimSpace(upstream) != "" {
		candidates = append(candidates, strings.TrimSpace(upstream))
	}
	if originHead, err := gitOutput(cwd, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil && strings.TrimSpace(originHead) != "" {
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
		if _, err := gitOutput(cwd, "rev-parse", "--verify", candidate+"^{commit}"); err != nil {
			continue
		}
		baseCommit, err := gitOutput(cwd, "merge-base", headCommit, candidate)
		if err != nil || strings.TrimSpace(baseCommit) == "" {
			continue
		}
		baseCommit = strings.TrimSpace(baseCommit)
		if baseCommit != headCommit {
			return baseCommit, nil
		}
	}

	if parentCommit, err := gitOutput(cwd, "rev-parse", headCommit+"^"); err == nil && strings.TrimSpace(parentCommit) != "" {
		return strings.TrimSpace(parentCommit), nil
	}

	return strings.TrimSpace(headCommit), nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (e *Executor) resolveRequestCommitPull(call message.ToolCallPart, req api.AnswerQuestionRequest) (message.ToolResultPart, error) {
	if strings.TrimSpace(req.Answers[requestCommitPullSucceededKey]) != "" {
		result := strings.TrimSpace(req.Answers[requestCommitPullResultKey])
		if result == "" {
			result = "Discobot successfully pulled the prepared sandbox commit into the host workspace."
		}
		return message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output:     message.TextOutput{Value: result},
		}, nil
	}
	if strings.TrimSpace(req.Answers[requestCommitPullFailedKey]) != "" {
		result := strings.TrimSpace(req.Answers[requestCommitPullResultKey])
		if result == "" {
			result = "Discobot failed to pull the prepared sandbox commit into the host workspace."
		}
		return message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output:     message.TextOutput{Value: result},
		}, nil
	}
	if strings.TrimSpace(req.Answers[requestCommitPullApprovedKey]) != "" {
		return message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output:     message.TextOutput{Value: requestCommitPullApprovedResponseText},
		}, nil
	}
	if strings.TrimSpace(req.Answers[requestCommitPullRejectedKey]) != "" {
		rejectionReason := strings.TrimSpace(req.Answers[requestCommitPullRejectedReasonKey])
		result := "The user rejected pulling the prepared sandbox commit into the host workspace."
		if rejectionReason != "" {
			result = fmt.Sprintf("%s Reason: %s", result, rejectionReason)
		}
		return message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output:     message.TextOutput{Value: result},
		}, nil
	}
	return message.ToolResultPart{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Output:     message.TextOutput{Value: "false"},
	}, nil
}
