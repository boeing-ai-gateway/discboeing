package exedev

import (
	"encoding/json"
	"time"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

type providerState struct {
	VMName       string         `json:"vmName,omitempty"`
	VMURL        string         `json:"vmUrl,omitempty"`
	VMAPIKey     string         `json:"vmApiKey,omitempty"`
	SharedSecret string         `json:"sharedSecret,omitempty"`
	Status       sandbox.Status `json:"status,omitempty"`
	CreatedAt    time.Time      `json:"createdAt,omitzero"`
}

func parseState(data []byte) providerState {
	var state providerState
	if len(data) == 0 {
		return state
	}
	_ = json.Unmarshal(data, &state)
	return state
}

func marshalState(state providerState) ([]byte, error) {
	if state.VMName == "" && state.VMURL == "" && state.VMAPIKey == "" && state.SharedSecret == "" && state.CreatedAt.IsZero() {
		return nil, nil
	}
	return json.Marshal(state)
}
