package toolrenderers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type BashView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type BashInput struct {
	Command         string              `json:"command"`
	Description     string              `json:"description"`
	Timeout         *int                `json:"timeout"`
	RunInBackground bool                `json:"run_in_background"`
	CredentialUses  []BashCredentialUse `json:"credentialUses"`
}

type BashCredentialUse struct {
	CredentialID string `json:"credentialId"`
	UseID        string `json:"useId"`
	EnvVar       string `json:"envVar"`
}

type BashOutput struct {
	Output   string `json:"output"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode *int   `json:"exitCode"`
}

type numberedOutputLine struct {
	LineNumber string
	Text       string
}

type parsedNumberedOutput struct {
	IsTruncated        bool
	TruncationFilePath string
	Lines              []numberedOutputLine
}

var (
	truncatedOutputPattern = regexp.MustCompile(`^\[Output too long \([^\]]+\)\. Full output written to: (.+)\]$`)
	numberedOutputPattern  = regexp.MustCompile(`^\s*(\d+)→(.*)$`)
)

func parseBashInput(input string) (BashInput, bool) {
	if strings.TrimSpace(input) == "" {
		return BashInput{}, false
	}
	var parsed BashInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return BashInput{}, false
	}
	return parsed, parsed.Command != "" || parsed.Description != ""
}

func parseBashOutput(output string) (BashOutput, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return BashOutput{}, false
	}
	var parsed BashOutput
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed, true
	}
	return BashOutput{Output: output}, true
}

func bashStdout(output BashOutput) string {
	if output.Output != "" {
		return output.Output
	}
	return output.Stdout
}

func bashExecutionError(view BashView, output BashOutput) string {
	if view.ErrorText != "" {
		return view.ErrorText
	}
	return output.Stderr
}

func bashHeader(input BashInput, inputOK bool, state string) string {
	if inputOK {
		if input.Description != "" {
			return input.Description
		}
		if input.Command != "" {
			return input.Command
		}
	}
	if isStreamingState(state) {
		return "Loading command details..."
	}
	return "Command"
}

func parseBashNumberedOutput(value string) parsedNumberedOutput {
	if value == "" {
		return parsedNumberedOutput{}
	}
	rawLines := strings.Split(normalizeNewlines(value), "\n")
	startIndex := 0
	result := parsedNumberedOutput{}
	for index, line := range rawLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if match := truncatedOutputPattern.FindStringSubmatch(line); match != nil {
			result.IsTruncated = true
			result.TruncationFilePath = strings.TrimSpace(match[1])
			startIndex = index + 1
			for startIndex < len(rawLines) && strings.TrimSpace(rawLines[startIndex]) == "" {
				startIndex++
			}
		}
		break
	}

	candidateLines := append([]string(nil), rawLines[startIndex:]...)
	for len(candidateLines) > 0 && candidateLines[len(candidateLines)-1] == "" {
		candidateLines = candidateLines[:len(candidateLines)-1]
	}
	if len(candidateLines) == 0 {
		return result
	}
	for _, line := range candidateLines {
		match := numberedOutputPattern.FindStringSubmatch(line)
		if match == nil {
			result.Lines = nil
			return result
		}
		result.Lines = append(result.Lines, numberedOutputLine{LineNumber: match[1], Text: match[2]})
	}
	return result
}

func bashLineCount(stdout string, parsed parsedNumberedOutput) int {
	if len(parsed.Lines) > 0 {
		return len(parsed.Lines)
	}
	if stdout == "" {
		return 0
	}
	return strings.Count(normalizeNewlines(stdout), "\n") + 1
}

func bashExitClass(output BashOutput) string {
	if output.ExitCode != nil && *output.ExitCode != 0 {
		return "bg-yellow-100 text-yellow-700"
	}
	return "bg-green-100 text-green-700"
}

func bashExitLabel(output BashOutput) string {
	if output.ExitCode == nil {
		return ""
	}
	return fmt.Sprintf("exit %d", *output.ExitCode)
}
