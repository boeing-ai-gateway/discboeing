package agentimpl

import (
	"reflect"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/scriptexec"
	"github.com/boeing-ai-gateway/discboeing/agent-go/sessionconfig"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func TestScriptExecutionMetadata(t *testing.T) {
	t.Parallel()

	execution := scriptexec.Execution{
		Script: sessionconfig.ScriptConfig{
			Name: "discboeing-commit",
			Path: "/tmp/discboeing-commit",
		},
		Stdout:   "stdout",
		Stderr:   "stderr",
		ExitCode: 17,
		Success:  true,
	}

	got := scriptExecutionMetadata(execution)
	want := &thread.ScriptExecutionMetadata{
		ScriptName: "discboeing-commit",
		ScriptPath: "/tmp/discboeing-commit",
		ExitCode:   17,
		Success:    true,
		Stdout:     "stdout",
		Stderr:     "stderr",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scriptExecutionMetadata() = %#v, want %#v", got, want)
	}
}

func TestDiscboeingCommandMetadata(t *testing.T) {
	t.Parallel()

	got := discboeingCommandMetadata(sessionconfig.DiscboeingCommandMetadata{
		UI:          true,
		Label:       "Commit",
		ActiveLabel: "Committing",
		Icon:        "git-commit",
		Group:       "Git",
		Order:       10,
		CredentialRequest: []sessionconfig.DiscboeingCredentialRequest{{
			EnvVar:        "GH_TOKEN",
			Name:          "GitHub credential",
			Justification: "Authenticate push.",
			ApprovedUses: []sessionconfig.DiscboeingCredentialApprovedUse{{
				Description: "push with gh",
			}},
		}},
	})
	want := api.CommandDiscboeingMetadata{
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
		t.Fatalf("discboeingCommandMetadata() = %#v, want %#v", got, want)
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
