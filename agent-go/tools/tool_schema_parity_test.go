package tools

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/sessionconfig"
)

func TestBuiltinToolSchemasMatchImplementationInputFields(t *testing.T) {
	t.Parallel()

	tools := sessionconfig.BuiltinTools("")
	expectedFields := map[string][]string{
		"AskUserQuestion":       jsonFieldNames(askUserQuestionInput{}),
		"Bash":                  jsonFieldNames(bashInput{}),
		"Edit":                  jsonFieldNames(editInput{}),
		"EnterPlanMode":         jsonFieldNames(struct{}{}),
		"ExitPlanMode":          jsonFieldNames(exitPlanModeInput{}),
		"Glob":                  jsonFieldNames(globInput{}),
		"Grep":                  jsonFieldNames(grepInput{}),
		"Read":                  jsonFieldNames(readInput{}),
		"RequestCommitPull":     jsonFieldNames(requestCommitPullInput{}),
		"RequestUserCredential": jsonFieldNames(requestUserCredentialInput{}),
		"Skill":                 jsonFieldNames(skillInput{}),
		"Task":                  jsonFieldNames(taskInput{}),
		"TaskOutput":            jsonFieldNames(taskOutputInput{}),
		"TaskStop":              jsonFieldNames(taskStopInput{}),
		"TodoWrite":             jsonFieldNames(todoWriteInput{}),
		"WebFetch":              jsonFieldNames(webFetchInput{}),
		"WebSearch":             jsonFieldNames(webSearchInput{}),
		"Write":                 jsonFieldNames(writeInput{}),
		"apply_patch":           jsonFieldNames(applyPatchInput{}),
	}

	for _, tool := range tools {
		wantFields, ok := expectedFields[tool.Name]
		if !ok {
			t.Fatalf("missing schema parity fixture for tool %q", tool.Name)
		}

		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatalf("%s schema: %v", tool.Name, err)
		}

		gotFields := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			gotFields = append(gotFields, name)
		}
		slices.Sort(gotFields)

		if !reflect.DeepEqual(gotFields, wantFields) {
			t.Errorf("%s schema properties = %v, want %v", tool.Name, gotFields, wantFields)
		}
	}
}

func jsonFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	names := make([]string, 0, t.NumField())
	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := tag
		if idx := strings.IndexByte(name, ','); idx >= 0 {
			name = name[:idx]
		}
		if name == "" {
			name = field.Name
		}
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}
