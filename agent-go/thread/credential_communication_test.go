package thread

import (
	"encoding/json"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

func TestRecordCommunicatedCredentialResult_MergesIntoThreadConfig(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread-1"

	if err := store.SaveConfig(threadID, Config{
		CommunicatedCredentials: []CommunicatedCredentialBinding{{
			CredentialID: "cred_s_old",
			EnvVar:       "OLD_TOKEN",
			Uses: []CommunicatedCredentialUse{{
				ID:          "use_s_old",
				Description: "old use",
			}},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	payload := struct {
		GrantedCredentials []api.GrantedCredential `json:"grantedCredentials"`
	}{
		GrantedCredentials: []api.GrantedCredential{{
			CredentialID: "cred_s_new",
			EnvVar:       "GH_TOKEN",
			Name:         "GitHub CLI token",
			ApprovedUses: []api.GrantedCredentialApprovedUse{{
				ID:          "use_s_new",
				Description: "authenticate gh",
			}},
		}},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	if err := recordCommunicatedCredentialResult(store, threadID, message.ToolResultPart{
		ToolCallID: "tc1",
		ToolName:   "RequestUserCredential",
		Output:     message.JSONOutput{Value: encoded},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CommunicatedCredentials) != 2 {
		t.Fatalf("expected 2 communicated credential bindings, got %#v", cfg.CommunicatedCredentials)
	}
	if cfg.CommunicatedCredentials[0].CredentialID != "cred_s_new" || cfg.CommunicatedCredentials[1].CredentialID != "cred_s_old" {
		t.Fatalf("unexpected communicated credentials %#v", cfg.CommunicatedCredentials)
	}
}
