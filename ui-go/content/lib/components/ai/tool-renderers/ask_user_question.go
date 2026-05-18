package toolrenderers

import (
	"encoding/json"
	"fmt"
	"strings"
)

type AskUserQuestionView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type AskUserQuestionInput struct {
	Questions []AskUserQuestionItem `json:"questions"`
}

type AskUserQuestionItem struct {
	Header      string                  `json:"header"`
	Question    string                  `json:"question"`
	MultiSelect bool                    `json:"multiSelect"`
	Options     []AskUserQuestionOption `json:"options"`
	Notes       string                  `json:"notes"`
}

type AskUserQuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Markdown    string `json:"markdown"`
}

func parseAskUserQuestionInput(input string) (AskUserQuestionInput, bool) {
	if strings.TrimSpace(input) == "" {
		return AskUserQuestionInput{}, false
	}
	var parsed AskUserQuestionInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return AskUserQuestionInput{}, false
	}
	return parsed, true
}

func parseAskUserQuestionAnswers(output string) map[string]string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}

	var object map[string]any
	if err := json.Unmarshal([]byte(trimmed), &object); err == nil {
		answers := make(map[string]string, len(object))
		for question, answer := range object {
			if value, ok := answer.(string); ok {
				answers[question] = value
			} else if answer != nil {
				answers[question] = fmt.Sprint(answer)
			}
		}
		if len(answers) > 0 {
			return answers
		}
	}

	var list []struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		answers := make(map[string]string, len(list))
		for _, item := range list {
			if item.Question != "" {
				answers[item.Question] = item.Answer
			}
		}
		if len(answers) > 0 {
			return answers
		}
	}

	answers := map[string]string{}
	for part := range strings.SplitSeq(trimmed, "\n") {
		question, answer, ok := strings.Cut(part, "=")
		if ok {
			question = strings.Trim(strings.TrimSpace(question), "\"")
			answer = strings.Trim(strings.TrimSpace(answer), "\"")
			if question != "" {
				answers[question] = answer
			}
		}
	}
	if len(answers) == 0 {
		return nil
	}
	return answers
}

func askUserQuestionOutputText(output string) string {
	return strings.TrimSpace(output)
}

func questionAnswer(answers map[string]string, question string) string {
	if answers == nil {
		return ""
	}
	if answer := answers[question]; answer != "" {
		return answer
	}
	return "No answer"
}

func firstQuestionNotes(questions []AskUserQuestionItem) string {
	for _, question := range questions {
		if strings.TrimSpace(question.Notes) != "" {
			return question.Notes
		}
	}
	return ""
}

func questionStepLabel(question AskUserQuestionItem, index int) string {
	if question.Header != "" {
		return question.Header
	}
	return fmt.Sprintf("Question %d", index+1)
}

func questionInputType(question AskUserQuestionItem) string {
	if question.MultiSelect {
		return "checkbox"
	}
	return "radio"
}
