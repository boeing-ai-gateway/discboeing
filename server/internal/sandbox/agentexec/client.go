package agentexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

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

const (
	streamFrameStdout = byte(1)
	streamFrameStderr = byte(2)
	streamBufferSize  = 512
)

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
	attachURL := "ws://sandbox/exec/" + url.PathEscape(id) + "/attach"
	conn, _, err := websocket.Dial(ctx, clientWebSocketURL(lease.Client, attachURL), &websocket.DialOptions{
		HTTPClient: lease.Client,
		HTTPHeader: clientHeaders(lease.Client),
	})
	if err != nil {
		return nil, err
	}
	stream := newStream(id, lease, conn)
	go stream.readLoop()
	return stream, nil
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
	stdout    *streamReader
	stderr    *streamReader
	stdoutCh  chan streamChunk
	stderrCh  chan streamChunk
	writeMu   sync.Mutex
	closeOnce sync.Once
}

type streamChunk struct {
	data []byte
}

type streamReader struct {
	ch  <-chan streamChunk
	buf []byte
}

func newStream(execID string, lease *sandbox.HTTPClientLease, conn *websocket.Conn) *Stream {
	stdoutCh := make(chan streamChunk, streamBufferSize)
	stderrCh := make(chan streamChunk, streamBufferSize)
	return &Stream{
		execID:   execID,
		lease:    lease,
		conn:     conn,
		stdout:   &streamReader{ch: stdoutCh},
		stderr:   &streamReader{ch: stderrCh},
		stdoutCh: stdoutCh,
		stderrCh: stderrCh,
	}
}

func (s *Stream) Read(buf []byte) (int, error) {
	return s.stdout.Read(buf)
}

func (s *Stream) Stderr() io.Reader { return s.stderr }

func (s *Stream) readLoop() {
	defer close(s.stdoutCh)
	defer close(s.stderrCh)
	for {
		msgType, payload, err := s.conn.Read(context.Background())
		if err != nil {
			return
		}
		if msgType != websocket.MessageBinary && msgType != websocket.MessageText {
			continue
		}
		s.dispatchOutput(payload)
	}
}

func (s *Stream) dispatchOutput(payload []byte) {
	if len(payload) == 0 {
		return
	}
	streamType := payload[0]
	data := payload[1:]
	chunk := streamChunk{data: append([]byte(nil), data...)}
	switch streamType {
	case streamFrameStdout:
		s.stdoutCh <- chunk
	case streamFrameStderr:
		s.stderrCh <- chunk
	}
}

func (r *streamReader) Read(buf []byte) (int, error) {
	if len(r.buf) > 0 {
		n := copy(buf, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}
	for {
		chunk, ok := <-r.ch
		if !ok {
			return 0, io.EOF
		}
		if len(chunk.data) == 0 {
			continue
		}
		n := copy(buf, chunk.data)
		r.buf = append(r.buf[:0], chunk.data[n:]...)
		return n, nil
	}
}

func (s *Stream) Write(data []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.conn.Write(context.Background(), websocket.MessageBinary, data); err != nil {
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
		err = s.conn.Close(websocket.StatusNormalClosure, "done")
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
