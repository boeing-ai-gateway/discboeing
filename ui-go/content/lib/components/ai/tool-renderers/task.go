package toolrenderers

import (
	"encoding/json"
	"strings"
)

type TaskView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type TaskInput struct {
	SubagentType    string `json:"subagent_type"`
	Description     string `json:"description"`
	Prompt          string `json:"prompt"`
	RunInBackground bool   `json:"run_in_background"`
}

type TaskOutput struct {
	AgentID    string `json:"agentId"`
	OutputFile string `json:"output_file"`
	Result     string `json:"result"`
	Error      string `json:"error"`
}

func parseTaskInput(input string) (TaskInput, bool) {
	if strings.TrimSpace(input) == "" {
		return TaskInput{}, false
	}
	var parsed TaskInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return TaskInput{}, false
	}
	return parsed, parsed.SubagentType != "" || parsed.Description != "" || parsed.Prompt != ""
}

func parseTaskOutput(output string) (TaskOutput, bool) {
	if strings.TrimSpace(output) == "" {
		return TaskOutput{}, false
	}
	var parsed TaskOutput
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		return parsed, true
	}
	return TaskOutput{}, false
}

func taskHeader(input TaskInput, inputOK bool, state string) string {
	if inputOK && strings.TrimSpace(input.Description) != "" {
		return strings.TrimSpace(input.Description)
	}
	if inputOK && strings.TrimSpace(input.SubagentType) != "" {
		return strings.TrimSpace(input.SubagentType)
	}
	if isStreamingState(state) {
		return "Loading task details..."
	}
	return "Sub-agent task"
}
