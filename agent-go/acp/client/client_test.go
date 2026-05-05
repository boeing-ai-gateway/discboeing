package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
)

type dialTransport struct {
	addr string
}

func (t dialTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return nil, err
	}
	return (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
}

func TestClientSessionLifecycleUsesRealJSONRPCServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	received := make(chan []string, 1)
	serverErr := make(chan error, 1)
	go func() {
		methods, err := serveLifecycle(ctx, ln)
		if err != nil {
			serverErr <- err
			return
		}
		received <- methods
		serverErr <- nil
	}()

	client, err := Connect(ctx, dialTransport{addr: ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	initResult, err := client.Initialize(ctx, protocol.InitializeRequest{
		ProtocolVersion: 4,
		ClientInfo: &protocol.Implementation{
			Name:    "discobot-agent-go",
			Version: "test",
		},
		ClientCapabilities: protocol.ClientCapabilities{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if initResult.ProtocolVersion != 4 {
		t.Fatalf("protocol version = %d, want 4", initResult.ProtocolVersion)
	}
	if initResult.AgentInfo == nil || initResult.AgentInfo.Name != "fake-acp-server" {
		t.Fatalf("agent info = %#v, want fake-acp-server", initResult.AgentInfo)
	}

	session, err := client.NewSession(ctx, protocol.NewSessionRequest{Cwd: "/workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if session.SessionID != "session-1" {
		t.Fatalf("session id = %q, want session-1", session.SessionID)
	}

	if _, err := client.LoadSession(ctx, protocol.LoadSessionRequest{
		Cwd:        "/workspace",
		MCPServers: []protocol.MCPServer{},
		SessionID:  session.SessionID,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.ResumeSession(ctx, protocol.ResumeSessionRequest{
		Cwd:        "/workspace",
		MCPServers: []protocol.MCPServer{},
		SessionID:  session.SessionID,
	}); err != nil {
		t.Fatal(err)
	}

	sessions, err := client.ListSessions(ctx, protocol.ListSessionsRequest{Cwd: new("/workspace")})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions.Sessions) != 1 || sessions.Sessions[0].SessionID != "session-1" {
		t.Fatalf("sessions = %#v, want session-1", sessions.Sessions)
	}

	var updates []protocol.SessionNotification
	promptResult, err := client.Prompt(ctx, protocol.PromptRequest{
		SessionID: session.SessionID,
		Prompt:    []protocol.ContentBlock{mustTextBlock(t, "hello ACP")},
	}, WithOnUpdate(func(notification protocol.SessionNotification) error {
		updates = append(updates, notification)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].SessionID != "session-1" {
		t.Fatalf("updates = %#v, want session-1 update", updates)
	}
	if promptResult.StopReason != protocol.StopReasonEndTurn {
		t.Fatalf("stop reason = %q, want %q", promptResult.StopReason, protocol.StopReasonEndTurn)
	}

	if err := client.Cancel(ctx, protocol.CancelNotification{SessionID: session.SessionID}); err != nil {
		t.Fatal(err)
	}

	select {
	case methods := <-received:
		want := []string{
			protocol.InitializeRequestMethod,
			protocol.NewSessionRequestMethod,
			protocol.LoadSessionRequestMethod,
			protocol.ResumeSessionRequestMethod,
			protocol.ListSessionsRequestMethod,
			protocol.PromptRequestMethod,
			protocol.CancelNotificationMethod,
		}
		if fmt.Sprint(methods) != fmt.Sprint(want) {
			t.Fatalf("methods = %v, want %v", methods, want)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestClientHandlesRequestPermissionDuringPrompt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptPermission(ctx, ln)
	}()

	client, err := Connect(ctx, dialTransport{addr: ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var permissionRequest protocol.RequestPermissionRequest
	result, err := client.Prompt(ctx, protocol.PromptRequest{
		SessionID: "session-1",
		Prompt:    []protocol.ContentBlock{mustTextBlock(t, "run command")},
	}, WithOnRequestPermission(func(_ context.Context, request protocol.RequestPermissionRequest) (protocol.RequestPermissionResponse, error) {
		permissionRequest = request
		return protocol.RequestPermissionResponse{
			Outcome: protocol.RequestPermissionOutcomeSelected{
				SelectedPermissionOutcome: protocol.SelectedPermissionOutcome{
					OptionID: "allow-once",
				},
			}.RequestPermissionOutcome(),
		}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != protocol.StopReasonEndTurn {
		t.Fatalf("stop reason = %q, want %q", result.StopReason, protocol.StopReasonEndTurn)
	}
	if permissionRequest.SessionID != "session-1" || permissionRequest.ToolCall.ToolCallID != "tool-1" {
		t.Fatalf("permission request = %#v, want session-1/tool-1", permissionRequest)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func serveLifecycle(ctx context.Context, ln net.Listener) ([]string, error) {
	conn, err := ln.Accept()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer rpcConn.Close()

	methods := make([]string, 0, 4)

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return nil, err
	}
	methods = append(methods, initReq.Method)
	if initReq.Params.ClientInfo == nil || initReq.Params.ClientInfo.Name != "discobot-agent-go" {
		return nil, fmt.Errorf("client info = %#v, want discobot-agent-go", initReq.Params.ClientInfo)
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{
		ProtocolVersion: 4,
		AgentInfo: &protocol.Implementation{
			Name:    "fake-acp-server",
			Version: "test",
		},
		AgentCapabilities: protocol.AgentCapabilities{
			SessionCapabilities: protocol.SessionCapabilities{},
		},
	}); err != nil {
		return nil, err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return nil, err
	}
	methods = append(methods, newReq.Method)
	if newReq.Params.Cwd != "/workspace" {
		return nil, fmt.Errorf("cwd = %q, want /workspace", newReq.Params.Cwd)
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "session-1"}); err != nil {
		return nil, err
	}

	loadReq, err := readRequest[protocol.LoadSessionRequest](ctx, rpcConn, protocol.LoadSessionRequestMethod)
	if err != nil {
		return nil, err
	}
	methods = append(methods, loadReq.Method)
	if loadReq.Params.Cwd != "/workspace" {
		return nil, fmt.Errorf("load cwd = %q, want /workspace", loadReq.Params.Cwd)
	}
	if loadReq.Params.SessionID != "session-1" {
		return nil, fmt.Errorf("load session id = %q, want session-1", loadReq.Params.SessionID)
	}
	if len(loadReq.Params.MCPServers) != 0 {
		return nil, fmt.Errorf("load mcp servers = %d, want 0", len(loadReq.Params.MCPServers))
	}
	if err := writeResponse(ctx, rpcConn, loadReq.ID, protocol.LoadSessionResponse{}); err != nil {
		return nil, err
	}

	resumeReq, err := readRequest[protocol.ResumeSessionRequest](ctx, rpcConn, protocol.ResumeSessionRequestMethod)
	if err != nil {
		return nil, err
	}
	methods = append(methods, resumeReq.Method)
	if resumeReq.Params.Cwd != "/workspace" {
		return nil, fmt.Errorf("resume cwd = %q, want /workspace", resumeReq.Params.Cwd)
	}
	if resumeReq.Params.SessionID != "session-1" {
		return nil, fmt.Errorf("resume session id = %q, want session-1", resumeReq.Params.SessionID)
	}
	if len(resumeReq.Params.MCPServers) != 0 {
		return nil, fmt.Errorf("resume mcp servers = %d, want 0", len(resumeReq.Params.MCPServers))
	}
	if err := writeResponse(ctx, rpcConn, resumeReq.ID, protocol.ResumeSessionResponse{}); err != nil {
		return nil, err
	}

	listReq, err := readRequest[protocol.ListSessionsRequest](ctx, rpcConn, protocol.ListSessionsRequestMethod)
	if err != nil {
		return nil, err
	}
	methods = append(methods, listReq.Method)
	if listReq.Params.Cwd == nil || *listReq.Params.Cwd != "/workspace" {
		return nil, fmt.Errorf("list cwd = %v, want /workspace", listReq.Params.Cwd)
	}
	if err := writeResponse(ctx, rpcConn, listReq.ID, protocol.ListSessionsResponse{
		Sessions: []protocol.SessionInfo{{SessionID: "session-1", Cwd: "/workspace"}},
	}); err != nil {
		return nil, err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return nil, err
	}
	methods = append(methods, promptReq.Method)
	if promptReq.Params.SessionID != "session-1" {
		return nil, fmt.Errorf("prompt session id = %q, want session-1", promptReq.Params.SessionID)
	}
	if len(promptReq.Params.Prompt) != 1 {
		return nil, fmt.Errorf("prompt block count = %d, want 1", len(promptReq.Params.Prompt))
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
			"content": map[string]any{
				"type": protocol.ContentBlockTextType,
				"text": "hello from update",
			},
		})),
	}); err != nil {
		return nil, err
	}
	if err := writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn}); err != nil {
		return nil, err
	}

	cancelReq, err := readRequest[protocol.CancelNotification](ctx, rpcConn, protocol.CancelNotificationMethod)
	if err != nil {
		return nil, err
	}
	methods = append(methods, cancelReq.Method)
	if cancelReq.ID.Raw() != nil {
		return nil, fmt.Errorf("cancel notification id = %v, want nil", cancelReq.ID.Raw())
	}
	if cancelReq.Params.SessionID != "session-1" {
		return nil, fmt.Errorf("cancel session id = %q, want session-1", cancelReq.Params.SessionID)
	}

	return methods, nil
}

func servePromptPermission(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	requestID, err := jsonrpc.MakeID(float64(99))
	if err != nil {
		return err
	}
	if err := writeRequest(ctx, rpcConn, requestID, protocol.RequestPermissionRequestMethod, protocol.RequestPermissionRequest{
		SessionID: "session-1",
		ToolCall: protocol.ToolCallUpdate{
			ToolCallID: "tool-1",
		},
		Options: []protocol.PermissionOption{{
			OptionID: "allow-once",
			Kind:     protocol.PermissionOptionKindAllowOnce,
			Name:     "Allow once",
		}},
	}); err != nil {
		return err
	}
	response, err := readResponse(ctx, rpcConn, requestID)
	if err != nil {
		return err
	}
	var permissionResponse protocol.RequestPermissionResponse
	if err := json.Unmarshal(response.Result, &permissionResponse); err != nil {
		return err
	}
	selected, ok := permissionResponse.Outcome.Variant().(protocol.RequestPermissionOutcomeSelected)
	if !ok || selected.OptionID != "allow-once" {
		return fmt.Errorf("permission response = %#v, want allow-once", permissionResponse.Outcome.Variant())
	}
	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

type receivedRequest[T any] struct {
	ID     jsonrpc.ID
	Method string
	Params T
}

func readRequest[T any](ctx context.Context, conn mcp.Connection, method string) (receivedRequest[T], error) {
	msg, err := conn.Read(ctx)
	if err != nil {
		return receivedRequest[T]{}, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return receivedRequest[T]{}, fmt.Errorf("unexpected JSON-RPC message %T", msg)
	}
	if req.Method != method {
		return receivedRequest[T]{}, fmt.Errorf("method = %q, want %q", req.Method, method)
	}
	var params T
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return receivedRequest[T]{}, err
	}
	return receivedRequest[T]{ID: req.ID, Method: req.Method, Params: params}, nil
}

func writeResponse(ctx context.Context, conn mcp.Connection, id jsonrpc.ID, result any) error {
	resp, err := response(id, result)
	if err != nil {
		return err
	}
	return conn.Write(ctx, resp)
}

func writeNotification(ctx context.Context, conn mcp.Connection, method string, params any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return conn.Write(ctx, &jsonrpc.Request{Method: method, Params: data})
}

func writeRequest(ctx context.Context, conn mcp.Connection, id jsonrpc.ID, method string, params any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return conn.Write(ctx, &jsonrpc.Request{ID: id, Method: method, Params: data})
}

func readResponse(ctx context.Context, conn mcp.Connection, id jsonrpc.ID) (*jsonrpc.Response, error) {
	msg, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	resp, ok := msg.(*jsonrpc.Response)
	if !ok {
		return nil, fmt.Errorf("unexpected JSON-RPC message %T", msg)
	}
	if resp.ID.Raw() != id.Raw() {
		return nil, fmt.Errorf("response id = %v, want %v", resp.ID.Raw(), id.Raw())
	}
	return resp, nil
}

func response(id jsonrpc.ID, result any) (*jsonrpc.Response, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &jsonrpc.Response{ID: id, Result: data}, nil
}

func mustTextBlock(t *testing.T, text string) protocol.ContentBlock {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"type": protocol.ContentBlockTextType,
		"text": text,
	})
	if err != nil {
		t.Fatal(err)
	}
	return protocol.NewContentBlockRaw(data)
}

func mustRawJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
