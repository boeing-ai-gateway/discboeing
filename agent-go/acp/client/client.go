package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
)

// Client is a minimal ACP JSON-RPC client built on the MCP SDK transport
// boundary. Reusing mcp.Transport lets ACP share the same stdio, command,
// in-memory, SSE, and streamable transport implementations as MCP while keeping
// ACP method names and payloads separate from the MCP protocol layer.
type Client struct {
	conn mcp.Connection
	mu   sync.Mutex
	next int64
}

// SessionUpdateHandler receives ACP session/update notifications interleaved
// with a request's response.
type SessionUpdateHandler func(protocol.SessionNotification) error

// RequestPermissionHandler handles ACP permission requests sent by the agent
// while another call, usually session/prompt, is in progress.
type RequestPermissionHandler func(context.Context, protocol.RequestPermissionRequest) (protocol.RequestPermissionResponse, error)

type callOptions struct {
	onUpdate            SessionUpdateHandler
	onRequestPermission RequestPermissionHandler
}

// CallOption configures how a JSON-RPC request is processed.
type CallOption func(*callOptions)

// WithOnUpdate reports session/update notifications interleaved with a request's
// response.
func WithOnUpdate(handler SessionUpdateHandler) CallOption {
	return func(opts *callOptions) {
		opts.onUpdate = handler
	}
}

// WithOnRequestPermission handles session/request_permission requests
// interleaved with a JSON-RPC call.
func WithOnRequestPermission(handler RequestPermissionHandler) CallOption {
	return func(opts *callOptions) {
		opts.onRequestPermission = handler
	}
}

// Connect creates an ACP client over an MCP SDK JSON-RPC transport.
func Connect(ctx context.Context, transport mcp.Transport) (*Client, error) {
	conn, err := transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// Close closes the underlying transport connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Initialize performs the ACP initialize JSON-RPC request.
func (c *Client) Initialize(ctx context.Context, params protocol.InitializeRequest, opts ...CallOption) (protocol.InitializeResponse, error) {
	var result protocol.InitializeResponse
	if err := c.call(ctx, protocol.InitializeRequestMethod, params, &result, opts...); err != nil {
		return protocol.InitializeResponse{}, err
	}
	return result, nil
}

// NewSession performs the ACP session/new JSON-RPC request.
func (c *Client) NewSession(ctx context.Context, params protocol.NewSessionRequest, opts ...CallOption) (protocol.NewSessionResponse, error) {
	var result protocol.NewSessionResponse
	if err := c.call(ctx, protocol.NewSessionRequestMethod, params, &result, opts...); err != nil {
		return protocol.NewSessionResponse{}, err
	}
	return result, nil
}

// LoadSession performs the ACP session/load JSON-RPC request.
func (c *Client) LoadSession(ctx context.Context, params protocol.LoadSessionRequest, opts ...CallOption) (protocol.LoadSessionResponse, error) {
	var result protocol.LoadSessionResponse
	if err := c.call(ctx, protocol.LoadSessionRequestMethod, params, &result, opts...); err != nil {
		return protocol.LoadSessionResponse{}, err
	}
	return result, nil
}

// ResumeSession performs the ACP session/resume JSON-RPC request.
func (c *Client) ResumeSession(ctx context.Context, params protocol.ResumeSessionRequest, opts ...CallOption) (protocol.ResumeSessionResponse, error) {
	var result protocol.ResumeSessionResponse
	if err := c.call(ctx, protocol.ResumeSessionRequestMethod, params, &result, opts...); err != nil {
		return protocol.ResumeSessionResponse{}, err
	}
	return result, nil
}

// ListSessions performs the ACP session/list JSON-RPC request.
func (c *Client) ListSessions(ctx context.Context, params protocol.ListSessionsRequest, opts ...CallOption) (protocol.ListSessionsResponse, error) {
	var result protocol.ListSessionsResponse
	if err := c.call(ctx, protocol.ListSessionsRequestMethod, params, &result, opts...); err != nil {
		return protocol.ListSessionsResponse{}, err
	}
	return result, nil
}

// Prompt performs the ACP session/prompt JSON-RPC request.
func (c *Client) Prompt(ctx context.Context, params protocol.PromptRequest, opts ...CallOption) (protocol.PromptResponse, error) {
	var result protocol.PromptResponse
	if err := c.call(ctx, protocol.PromptRequestMethod, params, &result, opts...); err != nil {
		return protocol.PromptResponse{}, err
	}
	return result, nil
}

// Cancel sends the ACP session/cancel JSON-RPC notification.
func (c *Client) Cancel(ctx context.Context, params protocol.CancelNotification) error {
	return c.notify(ctx, protocol.CancelNotificationMethod, params)
}

func (c *Client) call(ctx context.Context, method string, params, result any, opts ...CallOption) error {
	callOpts := callOptions{}
	for _, opt := range opts {
		opt(&callOpts)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.next++
	id, err := jsonrpc.MakeID(float64(c.next))
	if err != nil {
		return err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal %s params: %w", method, err)
	}

	if err := c.conn.Write(ctx, &jsonrpc.Request{
		ID:     id,
		Method: method,
		Params: paramsJSON,
	}); err != nil {
		return fmt.Errorf("write %s request: %w", method, err)
	}

	for {
		msg, err := c.conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read %s response: %w", method, err)
		}
		switch msg := msg.(type) {
		case *jsonrpc.Response:
			if msg.ID.Raw() != id.Raw() {
				return fmt.Errorf("read %s response: mismatched id %v", method, msg.ID.Raw())
			}
			if msg.Error != nil {
				return msg.Error
			}
			if result == nil {
				return nil
			}
			if err := json.Unmarshal(msg.Result, result); err != nil {
				return fmt.Errorf("decode %s result: %w", method, err)
			}
			return nil
		case *jsonrpc.Request:
			if msg.IsCall() {
				if err := c.handleRequest(ctx, method, msg, callOpts); err != nil {
					return err
				}
				continue
			}
			if msg.Method != protocol.SessionNotificationMethod {
				continue
			}
			if callOpts.onUpdate == nil {
				continue
			}
			var notification protocol.SessionNotification
			if err := json.Unmarshal(msg.Params, &notification); err != nil {
				return fmt.Errorf("decode %s notification: %w", protocol.SessionNotificationMethod, err)
			}
			if err := callOpts.onUpdate(notification); err != nil {
				return err
			}
		default:
			return fmt.Errorf("read %s response: expected response, got %T", method, msg)
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, activeMethod string, msg *jsonrpc.Request, opts callOptions) error {
	switch msg.Method {
	case protocol.RequestPermissionRequestMethod:
		if opts.onRequestPermission == nil {
			return fmt.Errorf("read %s response: unexpected server request %q", activeMethod, msg.Method)
		}
		var request protocol.RequestPermissionRequest
		if err := json.Unmarshal(msg.Params, &request); err != nil {
			return c.writeResponse(ctx, msg.ID, nil, fmt.Errorf("decode %s request: %w", msg.Method, err))
		}
		response, err := opts.onRequestPermission(ctx, request)
		if err != nil {
			return c.writeResponse(ctx, msg.ID, nil, err)
		}
		return c.writeResponse(ctx, msg.ID, response, nil)
	default:
		return fmt.Errorf("read %s response: unexpected server request %q", activeMethod, msg.Method)
	}
}

func (c *Client) writeResponse(ctx context.Context, id jsonrpc.ID, result any, responseErr error) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, &jsonrpc.Response{
		ID:     id,
		Result: data,
		Error:  responseErr,
	})
}

func (c *Client) notify(ctx context.Context, method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal %s params: %w", method, err)
	}
	if err := c.conn.Write(ctx, &jsonrpc.Request{
		Method: method,
		Params: paramsJSON,
	}); err != nil {
		return fmt.Errorf("write %s notification: %w", method, err)
	}
	return nil
}
