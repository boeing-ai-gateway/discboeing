package agentexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

type CreateRequest struct {
	Kind     string            `json:"kind,omitempty"`
	Name     string            `json:"name,omitempty"`
	ReuseKey string            `json:"reuseKey,omitempty"`
	Cmd      []string          `json:"cmd,omitempty"`
	WorkDir  string            `json:"workDir,omitempty"`
	HomeDir  bool              `json:"homeDir,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	User     string            `json:"user,omitempty"`
	TTY      bool              `json:"tty,omitempty"`
	Rows     int               `json:"rows,omitempty"`
	Cols     int               `json:"cols,omitempty"`
}

type Session struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exitCode,omitempty"`
}

type Event struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Error    string `json:"error,omitempty"`
}

type eventsResponse struct {
	Events []Event `json:"events"`
}

type ResizeRequest struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

func Create(ctx context.Context, client *http.Client, payload CreateRequest) (*Session, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://sandbox/exec", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError("agent exec create", resp)
	}
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func Get(ctx context.Context, client *http.Client, id string) (*Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://sandbox/exec/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError("agent exec get", resp)
	}
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func Events(ctx context.Context, client *http.Client, id string) ([]Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://sandbox/exec/"+url.PathEscape(id)+"/events", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError("agent exec events", resp)
	}
	var payload eventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Events, nil
}

func Wait(ctx context.Context, client *http.Client, id string) (*Session, error) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		session, err := Get(ctx, client, id)
		if err != nil {
			return nil, err
		}
		switch session.Status {
		case "exited", "killed", "failed":
			return session, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func Resize(ctx context.Context, client *http.Client, id string, rows, cols int) error {
	payload, err := json.Marshal(ResizeRequest{Rows: rows, Cols: cols})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://sandbox/exec/"+url.PathEscape(id)+"/resize", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("agent exec resize", resp)
	}
	return nil
}

func CloseWrite(ctx context.Context, client *http.Client, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://sandbox/exec/"+url.PathEscape(id)+"/close-write", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("agent exec close-write", resp)
	}
	return nil
}

func Kill(ctx context.Context, client *http.Client, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://sandbox/exec/"+url.PathEscape(id)+"/kill", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("agent exec kill", resp)
	}
	return nil
}

func Attach(ctx context.Context, lease *sandbox.HTTPClientLease, id string) (*Stream, error) {
	conn, _, err := dialer(lease.Client).DialContext(ctx, clientWebSocketURL(lease.Client, "ws://sandbox/exec/"+url.PathEscape(id)+"/attach"), clientHeaders(lease.Client))
	if err != nil {
		return nil, err
	}
	return &Stream{execID: id, lease: lease, conn: conn}, nil
}

func CreateAndAttach(ctx context.Context, lease *sandbox.HTTPClientLease, payload CreateRequest) (*Stream, error) {
	session, err := Create(ctx, lease.Client, payload)
	if err != nil {
		lease.Release()
		return nil, err
	}
	stream, err := Attach(ctx, lease, session.ID)
	if err != nil {
		_ = Kill(context.Background(), lease.Client, session.ID)
		lease.Release()
		return nil, err
	}
	return stream, nil
}

func dialer(client *http.Client) *websocket.Dialer {
	dialer := *websocket.DefaultDialer
	if transport, ok := client.Transport.(interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}); ok {
		dialer.Proxy = nil
		dialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return transport.DialContext(ctx, network, addr)
		}
	} else if transport, ok := client.Transport.(*http.Transport); ok && transport.DialContext != nil {
		dialer.Proxy = nil
		dialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return transport.DialContext(ctx, network, addr)
		}
	}
	return &dialer
}

func clientHeaders(client *http.Client) http.Header {
	if transport, ok := client.Transport.(interface{ Headers() http.Header }); ok {
		return transport.Headers()
	}
	return nil
}

func clientWebSocketURL(client *http.Client, rawURL string) string {
	if transport, ok := client.Transport.(interface{ WebSocketURL(string) string }); ok {
		return transport.WebSocketURL(rawURL)
	}
	return rawURL
}

type Stream struct {
	execID    string
	lease     *sandbox.HTTPClientLease
	conn      *websocket.Conn
	readBuf   []byte
	readMu    sync.Mutex
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func (s *Stream) Read(buf []byte) (int, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	if len(s.readBuf) > 0 {
		n := copy(buf, s.readBuf)
		s.readBuf = s.readBuf[n:]
		return n, nil
	}
	for {
		msgType, payload, err := s.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
			continue
		}
		n := copy(buf, payload)
		s.readBuf = append(s.readBuf[:0], payload[n:]...)
		return n, nil
	}
}

func (s *Stream) Stderr() io.Reader { return nil }

func (s *Stream) Write(data []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (s *Stream) Resize(ctx context.Context, rows, cols int) error {
	return Resize(ctx, s.lease.Client, s.execID, rows, cols)
}

func (s *Stream) CloseWrite() error {
	return CloseWrite(context.Background(), s.lease.Client, s.execID)
}

func (s *Stream) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.conn.Close()
		_ = Kill(context.Background(), s.lease.Client, s.execID)
		s.lease.Release()
	})
	return err
}

func (s *Stream) Wait(ctx context.Context) (int, error) {
	session, err := Wait(ctx, s.lease.Client, s.execID)
	if err != nil {
		return -1, err
	}
	if session.ExitCode != nil {
		return *session.ExitCode, nil
	}
	return 0, nil
}

func responseError(action string, resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("%s failed: status %d: %s", action, resp.StatusCode, strings.TrimSpace(string(data)))
}
