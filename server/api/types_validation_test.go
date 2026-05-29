package serverapi_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/server/client"
)

func TestGeneratedTypesMatchPublicClientDTOJSONFields(t *testing.T) {
	cases := []struct {
		name      string
		generated any
		existing  any
	}{
		{name: "ErrorResponse", generated: api.ErrorResponse{}, existing: client.ErrorResponse{}},
		{name: "HealthResponse", generated: api.HealthResponse{}, existing: client.HealthResponse{}},
		{name: "ServerConfig", generated: api.ServerConfig{}, existing: client.ServerConfig{}},
		{name: "Project", generated: api.Project{}, existing: client.Project{}},
		{name: "Workspace", generated: api.Workspace{}, existing: client.Workspace{}},
		{name: "Session", generated: api.Session{}, existing: client.Session{}},
		{name: "SessionThreadStatus", generated: api.SessionThreadStatus{}, existing: client.SessionThreadStatus{}},
		{name: "ModelInfo", generated: api.ModelInfo{}, existing: client.ModelInfo{}},
		{name: "Thread", generated: api.Thread{}, existing: client.Thread{}},
		{name: "QueuedPrompt", generated: api.QueuedPrompt{}, existing: client.QueuedPrompt{}},
		{name: "ThreadActivity", generated: api.ThreadActivity{}, existing: client.ThreadActivity{}},
		{name: "ThreadsResponse", generated: api.ThreadsResponse{}, existing: client.ThreadsResponse{}},
		{name: "ModelsResponse", generated: api.ModelsResponse{}, existing: client.ModelsResponse{}},
		{name: "ChatRequest", generated: api.ChatRequest{}, existing: client.ChatRequest{}},
		{name: "ChatResponse", generated: api.ChatResponse{}, existing: client.ChatResponse{}},
		{name: "Suggestion", generated: api.Suggestion{}, existing: client.Suggestion{}},
		{name: "StatusMessage", generated: api.StatusMessage{}, existing: client.StatusMessage{}},
		{name: "StartupTask", generated: api.StartupTask{}, existing: client.StartupTask{}},
		{name: "SystemStatusResponse", generated: api.SystemStatusResponse{}, existing: client.SystemStatusResponse{}},
		{name: "SupportInfoResponse", generated: api.SupportInfoResponse{}, existing: client.SupportInfoResponse{}},
		{name: "RuntimeInfo", generated: api.RuntimeInfo{}, existing: client.RuntimeInfo{}},
		{name: "ConfigInfo", generated: api.ConfigInfo{}, existing: client.ConfigInfo{}},
		{name: "VZInfo", generated: api.VZInfo{}, existing: client.VZInfo{}},
		{name: "DiskUsageInfo", generated: api.DiskUsageInfo{}, existing: client.DiskUsageInfo{}},
		{name: "DataDiskFileInfo", generated: api.DataDiskFileInfo{}, existing: client.DataDiskFileInfo{}},
		{name: "RouteInfo", generated: api.RouteInfo{}, existing: client.RouteInfo{}},
		{name: "RouteParam", generated: api.RouteParam{}, existing: client.RouteParam{}},
		{name: "CreateProjectRequest", generated: api.CreateProjectRequest{}, existing: client.CreateProjectRequest{}},
		{name: "UpdateProjectRequest", generated: api.UpdateProjectRequest{}, existing: client.UpdateProjectRequest{}},
		{name: "CreateWorkspaceRequest", generated: api.CreateWorkspaceRequest{}, existing: client.CreateWorkspaceRequest{}},
		{name: "ValidateWorkspaceRequest", generated: api.ValidateWorkspaceRequest{}, existing: client.ValidateWorkspaceRequest{}},
		{name: "ValidateWorkspaceResponse", generated: api.ValidateWorkspaceResponse{}, existing: client.ValidateWorkspaceResponse{}},
		{name: "UpdateWorkspaceRequest", generated: api.UpdateWorkspaceRequest{}, existing: client.UpdateWorkspaceRequest{}},
		{name: "CreateSessionRequest", generated: api.CreateSessionRequest{}, existing: client.CreateSessionRequest{}},
		{name: "UpdateSessionRequest", generated: api.UpdateSessionRequest{}, existing: client.UpdateSessionRequest{}},
		{name: "CreateThreadRequest", generated: api.CreateThreadRequest{}, existing: client.CreateThreadRequest{}},
		{name: "UpdateThreadRequest", generated: api.UpdateThreadRequest{}, existing: client.UpdateThreadRequest{}},
		{name: "CredentialVisibility", generated: api.CredentialVisibility{}, existing: client.CredentialVisibility{}},
		{name: "CredentialEnvVar", generated: api.CredentialEnvVar{}, existing: client.CredentialEnvVar{}},
		{name: "CreateCredentialRequest", generated: api.CreateCredentialRequest{}, existing: client.CreateCredentialRequest{}},
		{name: "AnthropicExchangeRequest", generated: api.AnthropicExchangeRequest{}, existing: client.AnthropicExchangeRequest{}},
		{name: "GitHubCopilotDeviceCodeRequest", generated: api.GitHubCopilotDeviceCodeRequest{}, existing: client.GitHubCopilotDeviceCodeRequest{}},
		{name: "GitHubCopilotPollRequest", generated: api.GitHubCopilotPollRequest{}, existing: client.GitHubCopilotPollRequest{}},
		{name: "GitHubCopilotDeviceCodeResponse", generated: api.GitHubCopilotDeviceCodeResponse{}, existing: client.GitHubCopilotDeviceCodeResponse{}},
		{name: "GitHubCopilotPollResponse", generated: api.GitHubCopilotPollResponse{}, existing: client.GitHubCopilotPollResponse{}},
		{name: "GitHubDeviceCodeRequest", generated: api.GitHubDeviceCodeRequest{}, existing: client.GitHubDeviceCodeRequest{}},
		{name: "GitHubAuthorizeRequest", generated: api.GitHubAuthorizeRequest{}, existing: client.GitHubAuthorizeRequest{}},
		{name: "GitHubAuthorizeResponse", generated: api.GitHubAuthorizeResponse{}, existing: client.GitHubAuthorizeResponse{}},
		{name: "GitHubPollRequest", generated: api.GitHubPollRequest{}, existing: client.GitHubPollRequest{}},
		{name: "GitHubExchangeRequest", generated: api.GitHubExchangeRequest{}, existing: client.GitHubExchangeRequest{}},
		{name: "GitHubExchangeResponse", generated: api.GitHubExchangeResponse{}, existing: client.GitHubExchangeResponse{}},
		{name: "GitHubCallbackStatusRequest", generated: api.GitHubCallbackStatusRequest{}, existing: client.GitHubCallbackStatusRequest{}},
		{name: "CodexDeviceCodeResponse", generated: api.CodexDeviceCodeResponse{}, existing: client.CodexDeviceCodeResponse{}},
		{name: "CodexAuthorizeRequest", generated: api.CodexAuthorizeRequest{}, existing: client.CodexAuthorizeRequest{}},
		{name: "CodexAuthorizeResponse", generated: api.CodexAuthorizeResponse{}, existing: client.CodexAuthorizeResponse{}},
		{name: "CodexExchangeRequest", generated: api.CodexExchangeRequest{}, existing: client.CodexExchangeRequest{}},
		{name: "CodexExchangeResponse", generated: api.CodexExchangeResponse{}, existing: client.CodexExchangeResponse{}},
		{name: "CodexPollRequest", generated: api.CodexPollRequest{}, existing: client.CodexPollRequest{}},
		{name: "CodexCallbackStatusRequest", generated: api.CodexCallbackStatusRequest{}, existing: client.CodexCallbackStatusRequest{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			generated := jsonFieldShapes(reflect.TypeOf(tc.generated))
			existing := jsonFieldShapes(reflect.TypeOf(tc.existing))
			if !reflect.DeepEqual(generated, existing) {
				t.Fatalf("JSON fields differ\ngenerated: %s\nexisting:  %s", formatFieldShapes(generated), formatFieldShapes(existing))
			}
		})
	}
}

func jsonFieldShapes(t reflect.Type) map[string]string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	fields := make(map[string]string, t.NumField())
	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			name = field.Name
		}
		if name == "-" {
			continue
		}
		fields[name] = jsonWireShape(field.Type)
	}
	return fields
}

func jsonWireShape(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == reflect.TypeFor[time.Time]() {
		return "string:date-time"
	}
	if t == reflect.TypeFor[json.RawMessage]() {
		return "any"
	}

	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array<" + jsonWireShape(t.Elem()) + ">"
	case reflect.Map, reflect.Struct:
		return "object"
	case reflect.Interface:
		return "any"
	default:
		return t.Kind().String()
	}
}

func formatFieldShapes(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%s", key, fields[key]))
	}
	return strings.Join(parts, ", ")
}
