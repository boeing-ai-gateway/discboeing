package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

// AskUserQuestion — pauses the turn and presents questions to the user.
// The LLM sends a questions array; we route this through the ApprovalRequest
// mechanism so the handler can surface it to the client.

type askUserQuestionInput struct {
	Questions   json.RawMessage `json:"questions"`
	Answers     json.RawMessage `json:"answers"`
	Annotations json.RawMessage `json:"annotations"`
	Metadata    json.RawMessage `json:"metadata"`
}

func (e *Executor) executeAskUserQuestion(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input askUserQuestionInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if len(input.Questions) == 0 {
		return errResult(call, "questions is required"), nil
	}

	return thread.ToolExecuteResult{
		Approval: &thread.ApprovalRequest{
			Questions: input.Questions,
		},
	}, nil
}

// resolveAskUserQuestion converts the user's answers back into a tool result.
// answers is a map of question → answer strings.
func (e *Executor) resolveAskUserQuestion(call message.ToolCallPart, req api.AnswerQuestionRequest) (message.ToolResultPart, error) {
	// Re-parse the original questions so we can format a nice result.
	var input askUserQuestionInput
	if err := json.Unmarshal([]byte(call.Input), &input); err != nil {
		return message.ToolResultPart{}, fmt.Errorf("re-parse AskUserQuestion input: %w", err)
	}

	// Merge questions with answers as JSON so the LLM can read them.
	type qaItem struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}

	// Parse the questions array to extract question texts.
	var questions []struct {
		Question string `json:"question"`
		Header   string `json:"header"`
	}
	_ = json.Unmarshal(input.Questions, &questions)

	// Build the merged Q&A output.
	merged := make([]qaItem, 0, len(req.Answers))
	for _, q := range questions {
		answer, ok := req.Answers[q.Question]
		if !ok {
			answer = req.Answers[q.Header]
		}
		merged = append(merged, qaItem{Question: q.Question, Answer: answer})
	}

	// Fall back: if no questions parsed, just include raw answers.
	if len(merged) == 0 {
		for q, a := range req.Answers {
			merged = append(merged, qaItem{Question: q, Answer: a})
		}
	}

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return message.ToolResultPart{}, fmt.Errorf("marshal Q&A: %w", err)
	}

	return message.ToolResultPart{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Output:     message.TextOutput{Value: string(out)},
	}, nil
}

type requestUserCredentialInput struct {
	Credentials json.RawMessage `json:"credentials"`
}

const (
	requestUserCredentialGrantedKey        = "__request_user_credential_granted__"
	requestUserCredentialRejectedKey       = "__request_user_credential_rejected__"
	requestUserCredentialRejectedReasonKey = "__request_user_credential_rejection_reason__"
)

func (e *Executor) executeRequestUserCredential(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input requestUserCredentialInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if len(input.Credentials) == 0 {
		return errResult(call, "credentials is required"), nil
	}

	normalized, err := normalizeRequestedCredentialsPayload(input.Credentials)
	if err != nil {
		return errResult(call, err.Error()), nil
	}

	return thread.ToolExecuteResult{
		Approval: &thread.ApprovalRequest{
			Credentials: normalized,
		},
	}, nil
}

func (e *Executor) resolveRequestUserCredential(call message.ToolCallPart, req api.AnswerQuestionRequest) (message.ToolResultPart, error) {
	if grantedJSON := strings.TrimSpace(req.Answers[requestUserCredentialGrantedKey]); grantedJSON != "" {
		var granted struct {
			GrantedCredentials []api.GrantedCredential `json:"grantedCredentials"`
		}
		if err := json.Unmarshal([]byte(grantedJSON), &granted); err != nil {
			return message.ToolResultPart{}, fmt.Errorf("invalid granted credentials payload: %w", err)
		}
		sort.Slice(granted.GrantedCredentials, func(i, j int) bool {
			return granted.GrantedCredentials[i].EnvVar < granted.GrantedCredentials[j].EnvVar
		})
		payload, err := json.Marshal(granted)
		if err != nil {
			return message.ToolResultPart{}, fmt.Errorf("marshal granted credentials payload: %w", err)
		}
		return message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output:     message.JSONOutput{Value: payload},
		}, nil
	}
	if len(req.Answers) > 0 && strings.TrimSpace(req.Answers[requestUserCredentialRejectedKey]) != "" {
		rejectionReason := strings.TrimSpace(req.Answers[requestUserCredentialRejectedReasonKey])
		result := "The user will not supply the requested credential."
		if rejectionReason != "" {
			result = fmt.Sprintf("The user will not supply the requested credential. Reason: %s", rejectionReason)
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

func normalizeRequestedCredentialsPayload(input json.RawMessage) (json.RawMessage, error) {
	var credentials []api.RequestedCredential
	if err := json.Unmarshal(input, &credentials); err != nil {
		return nil, fmt.Errorf("invalid credentials payload: %w", err)
	}

	normalized, err := normalizeRequestedCredentials(credentials)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal normalized credentials: %w", err)
	}
	return payload, nil
}

func normalizeRequestedCredentials(credentials []api.RequestedCredential) ([]api.RequestedCredential, error) {
	if len(credentials) == 0 {
		return nil, fmt.Errorf("credentials is required")
	}

	normalized := make([]api.RequestedCredential, 0, len(credentials))
	for i, cred := range credentials {
		cred.EnvVar = strings.TrimSpace(cred.EnvVar)
		cred.Name = strings.TrimSpace(cred.Name)
		cred.Justification = strings.TrimSpace(cred.Justification)
		for j := range cred.ApprovedUses {
			cred.ApprovedUses[j].Description = strings.TrimSpace(cred.ApprovedUses[j].Description)
			if cred.ApprovedUses[j].Description == "" {
				return nil, fmt.Errorf("credentials[%d].approvedUses[%d].description is required", i, j)
			}
		}

		if len(cred.ApprovedUses) == 0 {
			return nil, fmt.Errorf("credentials[%d].approvedUses is required", i)
		}
		if cred.EnvVar == "" {
			return nil, fmt.Errorf("credentials[%d].envVar is required", i)
		}
		if cred.Name == "" {
			return nil, fmt.Errorf("credentials[%d].name is required", i)
		}
		if cred.Justification == "" {
			return nil, fmt.Errorf("credentials[%d].justification is required", i)
		}

		normalized = append(normalized, cred)
	}

	return normalized, nil
}
