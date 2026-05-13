package controlsocket

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newRawSocketPair(t *testing.T) (*Conn, *websocket.Conn, func()) {
	t.Helper()

	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("accept websocket: %v", err)
			return
		}
		accepted <- ws
	}))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	clientWS, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		cancel()
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}

	var serverWS *websocket.Conn
	select {
	case serverWS = <-accepted:
	case <-ctx.Done():
		cancel()
		_ = clientWS.Close(websocket.StatusGoingAway, "timeout")
		server.Close()
		t.Fatal("timed out waiting for accepted websocket")
	}

	return NewConn(clientWS), serverWS, func() {
		cancel()
		_ = clientWS.CloseNow()
		_ = serverWS.CloseNow()
		server.Close()
	}
}

func TestStreamDataWritesBinaryWebSocketMessage(t *testing.T) {
	client, serverWS, cleanup := newRawSocketPair(t)
	defer cleanup()

	payload := []byte{0, 1, 2, 'h', 'e', 'l', 'l', 'o'}
	if err := client.WriteFrame(t.Context(), Frame{Channel: "git:binary", Type: TypeStreamData, Data: payload}); err != nil {
		t.Fatalf("write stream data: %v", err)
	}

	messageType, data, err := serverWS.Read(t.Context())
	if err != nil {
		t.Fatalf("read raw websocket message: %v", err)
	}
	if messageType != websocket.MessageBinary {
		t.Fatalf("message type = %v, want binary", messageType)
	}
	if bytes.Contains(data, []byte(`"data"`)) || bytes.Contains(data, []byte("AAECaGVsbG8=")) {
		t.Fatalf("binary stream data appears to contain JSON/base64: %q", data)
	}
	if !bytes.HasSuffix(data, payload) {
		t.Fatalf("binary message does not end with raw payload: %q", data)
	}
}

func TestReadFrameDecodesBinaryStreamData(t *testing.T) {
	client, serverWS, cleanup := newRawSocketPair(t)
	defer cleanup()

	payload := []byte("raw git pack bytes")
	data, err := encodeBinaryStreamFrame(Frame{Channel: "git:read", Type: TypeStreamData, Data: payload})
	if err != nil {
		t.Fatalf("encode binary stream frame: %v", err)
	}
	if err := serverWS.Write(t.Context(), websocket.MessageBinary, data); err != nil {
		t.Fatalf("write raw binary message: %v", err)
	}

	frame, err := client.ReadFrame(t.Context())
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if frame.Version != 1 || frame.Channel != "git:read" || frame.Type != TypeStreamData {
		t.Fatalf("unexpected frame metadata: %+v", frame)
	}
	if !bytes.Equal(frame.Data, payload) {
		t.Fatalf("frame data = %q, want %q", frame.Data, payload)
	}
}

func TestMetadataFramesRemainJSONTextMessages(t *testing.T) {
	client, serverWS, cleanup := newRawSocketPair(t)
	defer cleanup()

	if err := client.WriteFrame(t.Context(), Frame{Channel: "git:open", Type: TypeStreamOpen, Payload: Payload(map[string]string{"method": "POST"})}); err != nil {
		t.Fatalf("write metadata frame: %v", err)
	}

	messageType, data, err := serverWS.Read(t.Context())
	if err != nil {
		t.Fatalf("read raw websocket message: %v", err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("message type = %v, want text", messageType)
	}
	if !bytes.Contains(data, []byte(`"type":"stream.open"`)) {
		t.Fatalf("metadata message is not JSON stream.open: %s", data)
	}
}
