package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coder/websocket"

	"github.com/boeing-ai-gateway/discboeing/controlsocket"
)

// ControlSocket is a leased server-initiated WebSocket connection to agent-go.
type ControlSocket struct {
	*controlsocket.Conn
	release func()
}

func (c *ControlSocket) Close() error {
	if c == nil {
		return nil
	}
	if c.release != nil {
		defer c.release()
	}
	if c.Conn == nil {
		return nil
	}
	return c.Conn.Close()
}

// DialControlSocket connects from the server to the sandbox agent-api control
// WebSocket. The sandbox transport decides how the synthetic sandbox URL maps
// to the runtime-specific endpoint, preserving reverse-tunnel providers.
func (c *SandboxAgentClient) DialControlSocket(ctx context.Context, sessionID string) (*ControlSocket, error) {
	lease, err := c.acquireHTTPClient(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	release := func() {
		lease.Release()
	}

	headers := transportHeaders(lease.Client.Transport)
	rawURL := transportWebSocketURL(lease.Client.Transport, "ws://sandbox/control/ws")
	ws, resp, err := websocket.Dial(ctx, rawURL, &websocket.DialOptions{
		HTTPClient: lease.Client,
		HTTPHeader: headers,
	})
	if err != nil {
		release()
		if resp != nil {
			return nil, fmt.Errorf("dial sandbox control socket: status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("dial sandbox control socket: %w", err)
	}
	return &ControlSocket{Conn: controlsocket.NewConn(ws), release: release}, nil
}

func (s *SandboxService) dialControlSocket(ctx context.Context, sessionID string) (*ControlSocket, error) {
	return s.newAgentClient(ctx).DialControlSocket(ctx, sessionID)
}

func transportHeaders(transport http.RoundTripper) http.Header {
	if headerTransport, ok := transport.(interface{ Headers() http.Header }); ok {
		return cloneHeaders(headerTransport.Headers())
	}
	return nil
}

func transportWebSocketURL(transport http.RoundTripper, rawURL string) string {
	if websocketTransport, ok := transport.(interface{ WebSocketURL(string) string }); ok {
		return websocketTransport.WebSocketURL(rawURL)
	}
	return rawURL
}
