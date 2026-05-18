package toolrenderers

import (
	"encoding/json"
	"strings"
)

type SkillView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type SkillInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

type SkillOutput struct {
	Result string `json:"result"`
	Error  string `json:"error"`
}

func parseSkillInput(input string) (SkillInput, bool) {
	if strings.TrimSpace(input) == "" {
		return SkillInput{}, false
	}
	var parsed SkillInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return SkillInput{}, false
	}
	return parsed, strings.TrimSpace(parsed.Skill) != ""
}

func parseSkillOutput(output string) (SkillOutput, bool) {
	if strings.TrimSpace(output) == "" {
		return SkillOutput{}, false
	}
	var parsed SkillOutput
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		return parsed, true
	}
	return SkillOutput{}, false
}

func skillHeader(input SkillInput, inputOK bool, state string) string {
	if inputOK && input.Skill != "" {
		return input.Skill
	}
	if isStreamingState(state) {
		return "Loading skill..."
	}
	return "Skill"
}
