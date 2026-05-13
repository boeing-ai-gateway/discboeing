package controlsocket

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	shared "github.com/obot-platform/discobot/controlsocket"
)

// Client owns the agent side of the server-initiated sandbox control socket.
// It intentionally only knows about generic named changes and byte streams;
// feature-specific protocols are layered on top by callers.
type Client struct {
	connMu sync.RWMutex
	conn   *shared.Conn

	streamsMu sync.Mutex
	streams   map[string]*Stream
}

func New() *Client {
	return &Client{streams: map[string]*Stream{}}
}

func (c *Client) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		log.Printf("sandbox control socket upgrade failed: %v", err)
		return
	}
	conn := shared.NewConn(ws)

	c.connMu.Lock()
	old := c.conn
	c.conn = conn
	c.connMu.Unlock()
	if old != nil {
		_ = old.Close()
	}

	defer func() {
		c.connMu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.connMu.Unlock()
		_ = conn.Close()
		c.failStreams()
	}()

	for {
		frame, err := conn.ReadFrame(r.Context())
		if err != nil {
			return
		}
		c.dispatch(frame)
	}
}

// PostChange publishes a named control change with an arbitrary JSON payload.
func (c *Client) PostChange(ctx context.Context, name string, payload any) error {
	return c.write(ctx, shared.Frame{Channel: shared.ChannelControl, Type: shared.TypeChange, Name: name, Payload: shared.Payload(payload)})
}

// OpenStream opens a named byte stream. The channel name should identify the
// feature and request, for example "git:123".
func (c *Client) OpenStream(ctx context.Context, channel string, payload any) (*Stream, error) {
	if channel == "" {
		return nil, fmt.Errorf("stream channel is required")
	}
	stream := &Stream{client: c, channel: channel, frames: make(chan shared.Frame, 32)}
	c.streamsMu.Lock()
	if _, ok := c.streams[channel]; ok {
		c.streamsMu.Unlock()
		return nil, fmt.Errorf("stream channel %q is already open", channel)
	}
	c.streams[channel] = stream
	c.streamsMu.Unlock()

	if err := c.write(ctx, shared.Frame{Channel: channel, Type: shared.TypeStreamOpen, Payload: shared.Payload(payload)}); err != nil {
		c.removeStream(channel)
		return nil, err
	}
	return stream, nil
}

func (c *Client) write(ctx context.Context, frame shared.Frame) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return fmt.Errorf("sandbox control socket is not connected")
	}
	return conn.WriteFrame(ctx, frame)
}

func (c *Client) dispatch(frame shared.Frame) {
	if frame.Type == shared.TypeStreamData && len(frame.Data) > 0 {
		frame.Data = append([]byte(nil), frame.Data...)
	}
	c.streamsMu.Lock()
	stream := c.streams[frame.Channel]
	if stream != nil && frame.Type == shared.TypeStreamClose {
		delete(c.streams, frame.Channel)
		stream.markRemoteFullCloseQueued()
	}
	c.streamsMu.Unlock()
	if stream == nil {
		return
	}
	if frame.Type == shared.TypeStreamClose {
		stream.frames <- frame
		return
	}
	if stream.isClosed() {
		return
	}
	stream.frames <- frame
}

func (c *Client) removeStream(channel string) {
	c.streamsMu.Lock()
	delete(c.streams, channel)
	c.streamsMu.Unlock()
}

func (c *Client) failStreams() {
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()
	for _, stream := range c.streams {
		stream.markClosedWithError(fmt.Errorf("control stream error: control socket disconnected"))
		select {
		case stream.frames <- shared.Frame{Type: shared.TypeError, Payload: []byte(`{"error":"control socket disconnected"}`)}:
		default:
		}
	}
}

// Stream is a named byte stream over the control socket.
type Stream struct {
	client      *Client
	channel     string
	frames      chan shared.Frame
	buf         []byte
	mu          sync.Mutex
	closed      bool
	writeClosed bool
	readClosed  bool
	readErr     error
}

func (s *Stream) Channel() string { return s.channel }

func (s *Stream) Frames() <-chan shared.Frame { return s.frames }

func (s *Stream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(s.buf) == 0 {
		s.mu.Lock()
		if s.readClosed {
			err := s.readErr
			s.mu.Unlock()
			if err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		s.mu.Unlock()

		frame, ok := <-s.frames
		if !ok {
			return 0, io.EOF
		}
		switch frame.Type {
		case shared.TypeStreamData:
			s.buf = frame.Data
		case shared.TypeStreamCloseWrite, shared.TypeStreamClose:
			s.markReadClosed(nil)
			return 0, io.EOF
		case shared.TypeError:
			err := fmt.Errorf("control stream error: %s", string(frame.Payload))
			s.markReadClosed(err)
			return 0, err
		}
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

func (s *Stream) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := s.checkWritable(); err != nil {
		return 0, err
	}
	chunk := append([]byte(nil), p...)
	if err := s.client.write(context.Background(), shared.Frame{Channel: s.channel, Type: shared.TypeStreamData, Data: chunk}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *Stream) CloseWrite() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("control stream %q is closed", s.channel)
	}
	if s.writeClosed {
		s.mu.Unlock()
		return nil
	}
	s.writeClosed = true
	s.mu.Unlock()
	return s.client.write(context.Background(), shared.Frame{Channel: s.channel, Type: shared.TypeStreamCloseWrite})
}

func (s *Stream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.writeClosed = true
	s.readClosed = true
	s.mu.Unlock()
	s.client.removeStream(s.channel)
	return s.client.write(context.Background(), shared.Frame{Channel: s.channel, Type: shared.TypeStreamClose})
}

func (s *Stream) checkWritable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("control stream %q is closed", s.channel)
	}
	if s.writeClosed {
		return fmt.Errorf("control stream %q write side is closed", s.channel)
	}
	return nil
}

func (s *Stream) markRemoteFullCloseQueued() {
	s.mu.Lock()
	s.closed = true
	s.writeClosed = true
	s.mu.Unlock()
}

func (s *Stream) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Stream) markClosedWithError(err error) {
	s.mu.Lock()
	s.closed = true
	s.writeClosed = true
	s.readClosed = true
	s.readErr = err
	s.mu.Unlock()
}

func (s *Stream) markReadClosed(err error) {
	s.mu.Lock()
	s.readClosed = true
	s.readErr = err
	if err == nil {
		s.readErr = nil
	}
	s.mu.Unlock()
}
