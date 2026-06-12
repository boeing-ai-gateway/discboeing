package agentimpl

import (
	"reflect"
	"testing"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/scriptexec"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func TestScriptExecutionMetadata(t *testing.T) {
	t.Parallel()

	execution := scriptexec.Execution{
		Script: sessionconfig.ScriptConfig{
			Name: "discobot-commit",
			Path: "/tmp/discobot-commit",
		},
		Stdout:   "stdout",
		Stderr:   "stderr",
		ExitCode: 17,
		Success:  true,
	}

	got := scriptExecutionMetadata(execution)
	want := &thread.ScriptExecutionMetadata{
		ScriptName: "discobot-commit",
		ScriptPath: "/tmp/discobot-commit",
		ExitCode:   17,
		Success:    true,
		Stdout:     "stdout",
		Stderr:     "stderr",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scriptExecutionMetadata() = %#v, want %#v", got, want)
	}
}

func TestDiscobotCommandMetadata(t *testing.T) {
	t.Parallel()

	got := discobotCommandMetadata(sessionconfig.DiscobotCommandMetadata{
		UI:          true,
		Label:       "Commit",
		ActiveLabel: "Committing",
		Icon:        "git-commit",
		Group:       "Git",
		Order:       10,
		CredentialRequest: []sessionconfig.DiscobotCredentialRequest{{
			EnvVar:        "GH_TOKEN",
			Name:          "GitHub credential",
			Justification: "Authenticate push.",
			ApprovedUses: []sessionconfig.DiscobotCredentialApprovedUse{{
				Description: "push with gh",
			}},
		}},
	})
	want := api.CommandDiscobotMetadata{
		UI:          true,
		Label:       "Commit",
		ActiveLabel: "Committing",
		Icon:        "git-commit",
		Group:       "Git",
		Order:       10,
		CredentialRequest: []api.CommandCredentialRequest{{
			EnvVar:        "GH_TOKEN",
			Name:          "GitHub credential",
			Justification: "Authenticate push.",
			ApprovedUses: []api.CommandApprovedUse{{
				Description: "push with gh",
			}},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discobotCommandMetadata() = %#v, want %#v", got, want)
	}
}

func TestScriptExecutionMetadata_FieldCoverage(t *testing.T) {
	t.Parallel()

	assertExportedFieldsExact(t, reflect.TypeFor[scriptexec.Execution](), []string{
		"Script",
		"Stdout",
		"Stderr",
		"ExitCode",
		"Success",
	})
	assertExportedFieldsExact(t, reflect.TypeFor[thread.ScriptExecutionMetadata](), []string{
		"ScriptName",
		"ScriptPath",
		"ExitCode",
		"Success",
		"Stdout",
		"Stderr",
		"SuppressedLLM",
	})
}

func assertExportedFieldsExact(t *testing.T, typ reflect.Type, want []string) {
	t.Helper()

	fields := reflect.VisibleFields(typ)
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.PkgPath != "" {
			continue
		}
		got = append(got, field.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exported fields for %s = %v, want %v", typ, got, want)
	}
}
