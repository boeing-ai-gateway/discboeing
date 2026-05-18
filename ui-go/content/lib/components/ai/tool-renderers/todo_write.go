package toolrenderers

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TodoWriteView struct {
	Input         string
	Output        string
	ErrorText     string
	State         string
	Open          bool
	Raw           bool
	Queued        bool
	PreviousTodos []TodoEntry
}

type TodoWriteInput struct {
	Todos []TodoEntry `json:"todos"`
}

type TodoEntry struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status"`
}

type TodoWriteOutput struct {
	Success bool   `json:"success"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

type todoCounts struct {
	Completed  int
	InProgress int
	Pending    int
}

func parseTodoWriteInput(input string) (TodoWriteInput, bool) {
	if strings.TrimSpace(input) == "" {
		return TodoWriteInput{}, false
	}
	var parsed TodoWriteInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return TodoWriteInput{}, false
	}
	return parsed, parsed.Todos != nil
}

func parseTodoWriteOutput(output string) (TodoWriteOutput, bool) {
	if strings.TrimSpace(output) == "" {
		return TodoWriteOutput{}, false
	}
	var parsed TodoWriteOutput
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		return parsed, true
	}
	return TodoWriteOutput{}, false
}

func todoLabel(todo TodoEntry) string {
	if strings.TrimSpace(todo.ActiveForm) != "" {
		return strings.TrimSpace(todo.ActiveForm)
	}
	if strings.TrimSpace(todo.Content) != "" {
		return strings.TrimSpace(todo.Content)
	}
	return "Untitled task"
}

func todoContentLabel(todo TodoEntry) string {
	if strings.TrimSpace(todo.Content) != "" {
		return strings.TrimSpace(todo.Content)
	}
	return "Untitled task"
}

func todoKey(todo TodoEntry) string {
	return todo.Content + "::" + todo.ActiveForm + "::" + todo.Status
}

func todoWriteCounts(todos []TodoEntry) todoCounts {
	var counts todoCounts
	for _, todo := range todos {
		switch todo.Status {
		case "completed":
			counts.Completed++
		case "in_progress":
			counts.InProgress++
		default:
			counts.Pending++
		}
	}
	return counts
}

func todoWriteProgressPercent(todos []TodoEntry) int {
	if len(todos) == 0 {
		return 0
	}
	return int(float64(todoWriteCounts(todos).Completed)/float64(len(todos))*100 + 0.5)
}

func todoWriteCurrent(todos []TodoEntry) (TodoEntry, bool) {
	for _, todo := range todos {
		if todo.Status == "in_progress" {
			return todo, true
		}
	}
	for _, todo := range todos {
		if todo.Status != "completed" {
			return todo, true
		}
	}
	return TodoEntry{}, false
}

func todoCompletedToShow(todos []TodoEntry, previousTodos []TodoEntry) ([]TodoEntry, bool) {
	previousCompleted := map[string]struct{}{}
	for _, todo := range previousTodos {
		if todo.Status == "completed" {
			previousCompleted[todoKey(todo)] = struct{}{}
		}
	}

	var completed []TodoEntry
	var newlyCompleted []TodoEntry
	for _, todo := range todos {
		if todo.Status != "completed" {
			continue
		}
		completed = append(completed, todo)
		if _, ok := previousCompleted[todoKey(todo)]; !ok {
			newlyCompleted = append(newlyCompleted, todo)
		}
	}
	if len(newlyCompleted) > 0 {
		return newlyCompleted, true
	}
	if len(completed) > 2 {
		completed = completed[len(completed)-2:]
	}
	return completed, false
}

func todoWriteHeader(todos []TodoEntry, inputOK bool, state string) string {
	if inputOK && len(todos) > 0 {
		counts := todoWriteCounts(todos)
		progress := fmt.Sprintf("%d/%d done", counts.Completed, len(todos))
		if current, ok := todoWriteCurrent(todos); ok {
			return progress + " • " + todoLabel(current)
		}
		return progress
	}
	if isStreamingState(state) {
		return "Loading todos..."
	}
	return "Todo write"
}
