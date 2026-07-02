package controlsocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	shared "github.com/boeing-ai-gateway/discboeing/controlsocket"
)

func newTestSocket(t *testing.T) (*Client, *shared.Conn, func()) {
	t.Helper()

	client := New()
	server := httptest.NewServer(httpHandler{client: client})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		cancel()
		server.Close()
		t.Fatalf("dial control socket: %v", err)
	}
	conn := shared.NewConn(ws)

	deadline := time.Now().Add(5 * time.Second)
	for {
		client.connMu.RLock()
		connected := client.conn != nil
		client.connMu.RUnlock()
		if connected {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			_ = conn.Close()
			server.Close()
			t.Fatal("timed out waiting for accepted socket")
		}
		time.Sleep(10 * time.Millisecond)
	}

	return client, conn, func() {
		cancel()
		_ = conn.Close()
		server.Close()
	}
}

type httpHandler struct{ client *Client }

func (h httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.client.ServeWebSocket(w, r)
}

func TestPostChangeWritesNamedControlFrame(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	payload := map[string]string{"status": "running"}
	if err := client.PostChange(t.Context(), "test.change", payload); err != nil {
		t.Fatalf("post change: %v", err)
	}

	frame, err := server.ReadFrame(t.Context())
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if frame.Version != 1 || frame.Channel != shared.ChannelControl || frame.Type != shared.TypeChange || frame.Name != "test.change" {
		t.Fatalf("unexpected frame: %+v", frame)
	}
	var got map[string]string
	if err := json.Unmarshal(frame.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got["status"] != "running" {
		t.Fatalf("expected status payload, got %+v", got)
	}
}

func TestOpenStreamWritesOpenFrameAndRejectsDuplicate(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:1", map[string]string{"method": "POST"})
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if stream.Channel() != "git:1" {
		t.Fatalf("expected channel git:1, got %q", stream.Channel())
	}

	frame, err := server.ReadFrame(t.Context())
	if err != nil {
		t.Fatalf("read open: %v", err)
	}
	if frame.Channel != "git:1" || frame.Type != shared.TypeStreamOpen {
		t.Fatalf("unexpected open frame: %+v", frame)
	}

	if _, err := client.OpenStream(t.Context(), "git:1", nil); err == nil {
		t.Fatal("expected duplicate stream error")
	}
}

func TestOpenStreamRemovesPendingStreamWhenWriteFails(t *testing.T) {
	client := New()
	if _, err := client.OpenStream(t.Context(), "git:1", nil); err == nil {
		t.Fatal("expected open without connection to fail")
	}
	client.streamsMu.Lock()
	_, ok := client.streams["git:1"]
	client.streamsMu.Unlock()
	if ok {
		t.Fatal("failed open left stream registered")
	}
}

func TestStreamWriteCopiesInputBuffer(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:copy", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	buf := []byte("original")
	if n, err := stream.Write(buf); err != nil || n != len(buf) {
		t.Fatalf("write = %d, %v", n, err)
	}
	copy(buf, "mutated!")

	frame, err := server.ReadFrame(t.Context())
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	if frame.Type != shared.TypeStreamData || string(frame.Data) != "original" {
		t.Fatalf("write did not copy input, frame=%+v data=%q", frame, string(frame.Data))
	}
}

func TestStreamReadPreservesUnreadBytesAcrossSmallReads(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:read", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:read", Type: shared.TypeStreamData, Data: []byte("abcdef")}); err != nil {
		t.Fatalf("write remote data: %v", err)
	}

	buf := make([]byte, 2)
	var got bytes.Buffer
	for range 3 {
		n, err := stream.Read(buf)
		if err != nil {
			t.Fatalf("read chunk: %v", err)
		}
		got.Write(buf[:n])
	}
	if got.String() != "abcdef" {
		t.Fatalf("expected preserved bytes, got %q", got.String())
	}
}

func TestDispatchCopiesInboundDataBeforeQueueing(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:inbound-copy", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	data := []byte("original")
	client.dispatch(shared.Frame{Channel: "git:inbound-copy", Type: shared.TypeStreamData, Data: data})
	copy(data, "mutated!")

	buf := make([]byte, len("original"))
	n, err := io.ReadFull(stream, buf)
	if err != nil || n != len(buf) || string(buf) != "original" {
		t.Fatalf("read inbound data = n=%d data=%q err=%v", n, string(buf), err)
	}
}

func TestDispatchBackpressuresInsteadOfDroppingStreamData(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:backpressure", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	const chunks = 128
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for i := range chunks {
			client.dispatch(shared.Frame{Channel: "git:backpressure", Type: shared.TypeStreamData, Data: []byte{byte(i)}})
		}
		client.dispatch(shared.Frame{Channel: "git:backpressure", Type: shared.TypeStreamCloseWrite})
	}()

	var got []byte
	buf := make([]byte, 1)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
	}
	<-writeDone

	if len(got) != chunks {
		t.Fatalf("read %d chunks, want %d", len(got), chunks)
	}
	for i, value := range got {
		if value != byte(i) {
			t.Fatalf("chunk %d = %d, want %d", i, value, byte(i))
		}
	}
}

func TestStreamReadSkipsMetadataFramesUntilDataOrClose(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:metadata", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:metadata", Type: shared.TypeStreamOpen, Payload: shared.Payload(map[string]int{"status": 200})}); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:metadata", Type: shared.TypeStreamData, Data: []byte("body")}); err != nil {
		t.Fatalf("write data: %v", err)
	}

	buf := make([]byte, len("body"))
	n, err := io.ReadFull(stream, buf)
	if err != nil || n != len(buf) || string(buf) != "body" {
		t.Fatalf("read after metadata = n=%d data=%q err=%v", n, string(buf), err)
	}
}

func TestStreamReadReturnsBufferedBytesBeforeRemoteCloseWriteEOF(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:half-close-read", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:half-close-read", Type: shared.TypeStreamData, Data: []byte("abc")}); err != nil {
		t.Fatalf("write remote data: %v", err)
	}
	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:half-close-read", Type: shared.TypeStreamCloseWrite}); err != nil {
		t.Fatalf("write remote close write: %v", err)
	}

	buf := make([]byte, 2)
	n, err := stream.Read(buf)
	if err != nil || string(buf[:n]) != "ab" {
		t.Fatalf("first read = %q, %v", string(buf[:n]), err)
	}
	n, err = stream.Read(buf)
	if err != nil || string(buf[:n]) != "c" {
		t.Fatalf("second read = %q, %v", string(buf[:n]), err)
	}
	n, err = stream.Read(buf)
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after buffered bytes, got n=%d err=%v", n, err)
	}
}

func TestCloseWriteHalfClosesWriteSideButStillAllowsRead(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:half-close-write", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	if err := stream.CloseWrite(); err != nil {
		t.Fatalf("close write: %v", err)
	}
	frame, err := server.ReadFrame(t.Context())
	if err != nil {
		t.Fatalf("read close write: %v", err)
	}
	if frame.Channel != "git:half-close-write" || frame.Type != shared.TypeStreamCloseWrite {
		t.Fatalf("unexpected close-write frame: %+v", frame)
	}

	if _, err := stream.Write([]byte("late write")); err == nil {
		t.Fatal("expected write after close-write to fail")
	}
	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:half-close-write", Type: shared.TypeStreamData, Data: []byte("response")}); err != nil {
		t.Fatalf("write response: %v", err)
	}
	buf := make([]byte, len("response"))
	n, err := io.ReadFull(stream, buf)
	if err != nil || n != len(buf) || string(buf) != "response" {
		t.Fatalf("read response after close-write = n=%d data=%q err=%v", n, string(buf), err)
	}
}

func TestCloseRemovesStreamAndRejectsFurtherWrites(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:close", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	frame, err := server.ReadFrame(t.Context())
	if err != nil {
		t.Fatalf("read close: %v", err)
	}
	if frame.Channel != "git:close" || frame.Type != shared.TypeStreamClose {
		t.Fatalf("unexpected close frame: %+v", frame)
	}

	client.streamsMu.Lock()
	_, ok := client.streams["git:close"]
	client.streamsMu.Unlock()
	if ok {
		t.Fatal("closed stream remained registered")
	}
	if _, err := stream.Write([]byte("late write")); err == nil {
		t.Fatal("expected write after close to fail")
	}
	if err := stream.CloseWrite(); err == nil {
		t.Fatal("expected close-write after close to fail")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second close should be idempotent: %v", err)
	}
}

func TestRemoteFullCloseRemovesStream(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:remote-close", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:remote-close", Type: shared.TypeStreamClose}); err != nil {
		t.Fatalf("write remote close: %v", err)
	}
	buf := make([]byte, 1)
	if n, err := stream.Read(buf); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF from remote close, got n=%d err=%v", n, err)
	}

	client.streamsMu.Lock()
	_, ok := client.streams["git:remote-close"]
	client.streamsMu.Unlock()
	if ok {
		t.Fatal("remote full close left stream registered")
	}
}

func TestRemoteCloseWriteDoesNotRemoveStream(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:remote-close-write", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	if err := server.WriteFrame(t.Context(), shared.Frame{Channel: "git:remote-close-write", Type: shared.TypeStreamCloseWrite}); err != nil {
		t.Fatalf("write remote close write: %v", err)
	}
	buf := make([]byte, 1)
	if n, err := stream.Read(buf); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF from remote close-write, got n=%d err=%v", n, err)
	}

	client.streamsMu.Lock()
	_, ok := client.streams["git:remote-close-write"]
	client.streamsMu.Unlock()
	if !ok {
		t.Fatal("remote close-write should leave stream registered for local writes")
	}
	if _, err := stream.Write([]byte("still writing")); err != nil {
		t.Fatalf("local write after remote close-write should work: %v", err)
	}
}

func TestDisconnectReportsErrorToPendingStream(t *testing.T) {
	client, server, cleanup := newTestSocket(t)
	defer cleanup()

	stream, err := client.OpenStream(t.Context(), "git:disconnect", nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	_, _ = server.ReadFrame(t.Context()) // stream.open

	_ = server.Close()
	buf := make([]byte, 1)
	n, err := stream.Read(buf)
	if n != 0 || err == nil || !strings.Contains(err.Error(), "control socket disconnected") {
		t.Fatalf("expected disconnect error, got n=%d err=%v", n, err)
	}
}
