package recorder

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureStream_UnderLimit(t *testing.T) {
	body := []byte("hello world")
	rc := io.NopCloser(bytes.NewReader(body))

	captured, restored, truncated := captureStream(rc, 100)

	if truncated {
		t.Error("expected truncated=false for body smaller than limit")
	}
	if !bytes.Equal(captured, body) {
		t.Errorf("captured = %q, want %q", captured, body)
	}
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want %q", got, body)
	}
}

func TestCaptureStream_ExactLimit(t *testing.T) {
	body := []byte("exactly ten!")
	rc := io.NopCloser(bytes.NewReader(body))

	captured, restored, truncated := captureStream(rc, int64(len(body)))

	if truncated {
		t.Error("expected truncated=false for body equal to limit")
	}
	if !bytes.Equal(captured, body) {
		t.Errorf("captured = %q, want %q", captured, body)
	}
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want %q", got, body)
	}
}

// TestCaptureStream_OverLimit is the regression test for the bug where the
// restored stream was missing the first maxSize bytes when the body exceeded
// the capture limit, causing downstream clients (e.g. Docker) to receive a
// truncated body and fail with "unexpected EOF".
func TestCaptureStream_OverLimit(t *testing.T) {
	body := []byte("abcdefghij") // 10 bytes
	rc := io.NopCloser(bytes.NewReader(body))
	const maxSize = 4

	captured, restored, truncated := captureStream(rc, maxSize)

	if !truncated {
		t.Error("expected truncated=true for body larger than limit")
	}
	if !bytes.Equal(captured, body[:maxSize]) {
		t.Errorf("captured = %q, want %q", captured, body[:maxSize])
	}
	// The restored stream must contain the full original body, not just the
	// bytes from position maxSize onward.
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want full body %q", got, body)
	}
}

func TestCaptureStream_Unlimited(t *testing.T) {
	body := []byte("some large content")
	rc := io.NopCloser(bytes.NewReader(body))

	captured, restored, truncated := captureStream(rc, -1)

	if truncated {
		t.Error("expected truncated=false for unlimited capture")
	}
	if !bytes.Equal(captured, body) {
		t.Errorf("captured = %q, want %q", captured, body)
	}
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want %q", got, body)
	}
}

func TestCaptureStream_Empty(t *testing.T) {
	rc := io.NopCloser(bytes.NewReader(nil))

	captured, restored, truncated := captureStream(rc, 100)

	if truncated {
		t.Error("expected truncated=false for empty body")
	}
	if len(captured) != 0 {
		t.Errorf("captured = %q, want empty", captured)
	}
	got, _ := io.ReadAll(restored)
	if len(got) != 0 {
		t.Errorf("restored stream = %q, want empty", got)
	}
}

func TestIsBinaryContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/webp", true},
		{"video/mp4", true},
		{"audio/mpeg", true},
		{"font/woff2", true},
		{"application/octet-stream", true},
		{"application/zip", true},
		{"application/gzip", true},
		{"application/x-gzip", true},
		{"application/x-tar", true},
		{"application/x-bz2", true},
		{"application/x-xz", true},
		{"application/zstd", true},
		{"application/x-zstd", true},
		{"application/pdf", true},
		{"application/wasm", true},
		{"application/vnd.docker.image.rootfs.diff.tar.gzip", true},
		{"application/vnd.oci.image.layer.v1.tar+gzip", true},
		// Text / structured types must NOT be skipped.
		{"application/json", false},
		{"application/json; charset=utf-8", false},
		{"text/plain", false},
		{"text/html; charset=utf-8", false},
		{"application/x-www-form-urlencoded", false},
		{"application/xml", false},
		// Empty content-type should not be treated as binary.
		{"", false},
		// Case-insensitive.
		{"IMAGE/PNG", true},
		{"Application/Zip", true},
	}
	for _, tt := range tests {
		got := isBinaryContentType(tt.ct)
		if got != tt.want {
			t.Errorf("isBinaryContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

func TestCaptureRequestBody_SkipsBinaryContentType(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("PNG\x89binary data")
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "image/png")
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if len(entry.Request.Body) != 0 {
		t.Errorf("expected no body captured for binary content-type, got %d bytes", len(entry.Request.Body))
	}
	// Body stream must still be intact for downstream use.
	got, _ := io.ReadAll(req.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("request body not restored: got %q, want %q", got, body)
	}
}

func TestCaptureRequestBody_SkipsNullBytes(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("some\x00binary\x00data")
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/octet-stream")
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if len(entry.Request.Body) != 0 {
		t.Errorf("expected no body captured when null bytes present, got %d bytes", len(entry.Request.Body))
	}
}

func TestCaptureRequestBody_NullBytesStreamRestored(t *testing.T) {
	// Even when null bytes cause the capture to be discarded, the request body
	// must remain readable by downstream handlers.
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("hello\x00world")
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "text/plain") // not binary MIME, so passes first check
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if len(entry.Request.Body) != 0 {
		t.Errorf("expected no body captured when null bytes present, got %d bytes", len(entry.Request.Body))
	}
	got, _ := io.ReadAll(req.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("request body not restored after null-byte discard: got %q, want %q", got, body)
	}
}

func TestCaptureResponseBody_SkipsBinaryContentType(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("\x1f\x8b binary gz content")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/gzip"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	entry := &Entry{Response: &ResponseInfo{}}

	r.CaptureResponseBody(entry, resp)

	if len(entry.Response.Body) != 0 {
		t.Errorf("expected no body captured for binary content-type, got %d bytes", len(entry.Response.Body))
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("response body not restored: got %q, want %q", got, body)
	}
}

func TestCaptureRequestBody_TextNotSkipped(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte(`{"key":"value"}`)
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if !bytes.Equal(entry.Request.Body, body) {
		t.Errorf("expected body captured for text content-type, got %q", entry.Request.Body)
	}
}

func TestBeginResponseCapture_StreamsWithinLimit(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: 5}}
	entry := &Entry{Response: &ResponseInfo{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	capture := r.BeginResponseCapture(entry, resp)
	if capture == nil {
		t.Fatal("expected streaming response capture")
	}

	capture.Write([]byte("he"))
	capture.Write([]byte("llo"))
	capture.Write([]byte(" world"))
	capture.Finish()

	if !bytes.Equal(entry.Response.Body, []byte("hello")) {
		t.Fatalf("captured body = %q, want %q", entry.Response.Body, "hello")
	}
	if !entry.Response.BodyTruncated {
		t.Fatal("expected truncated body flag after exceeding limit")
	}
}

func TestBeginResponseCapture_DropsBinaryPayloadsDetectedByNullByte(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	entry := &Entry{Response: &ResponseInfo{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	capture := r.BeginResponseCapture(entry, resp)
	if capture == nil {
		t.Fatal("expected streaming response capture")
	}

	capture.Write([]byte("hello"))
	capture.Write([]byte{'\x00'})
	capture.Write([]byte("world"))
	capture.Finish()

	if len(entry.Response.Body) != 0 {
		t.Fatalf("expected null-byte payload to be discarded, got %q", entry.Response.Body)
	}
	if entry.Response.BodyTruncated {
		t.Fatal("expected truncated flag to remain false when payload is discarded")
	}
}

func TestBeginUpgradeStream_WritesBidirectionalFrames(t *testing.T) {
	r, err := New(Config{Enabled: true, Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer r.Close()

	entry := &Entry{}
	session, err := r.BeginUpgradeStream(entry, "websocket")
	if err != nil {
		t.Fatalf("BeginUpgradeStream failed: %v", err)
	}

	session.RecordChunk(UpgradeStreamClientToServer, []byte("ping"))
	session.RecordChunk(UpgradeStreamServerToClient, []byte("pong"))
	if err := session.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !entry.Upgrade {
		t.Fatal("expected upgrade flag on entry")
	}
	if entry.UpgradeType != "websocket" {
		t.Fatalf("entry.UpgradeType = %q, want websocket", entry.UpgradeType)
	}
	if entry.StreamRecord == nil {
		t.Fatal("expected stream record metadata on entry")
	}

	frames := readUpgradeStreamFramesForTest(t, filepath.Join(r.cfg.Dir, filepath.FromSlash(entry.StreamRecord.File)))
	if len(frames) != 2 {
		t.Fatalf("len(frames) = %d, want 2", len(frames))
	}
	if frames[0].Direction != byte(UpgradeStreamClientToServer) || string(frames[0].Payload) != "ping" {
		t.Fatalf("unexpected first frame: direction=%d payload=%q", frames[0].Direction, frames[0].Payload)
	}
	if frames[1].Direction != byte(UpgradeStreamServerToClient) || string(frames[1].Payload) != "pong" {
		t.Fatalf("unexpected second frame: direction=%d payload=%q", frames[1].Direction, frames[1].Payload)
	}
}

func TestUpgradeStreamSession_DropsChunksWhenWriterBlocks(t *testing.T) {
	writer := &blockingWriteCloser{allowWrites: make(chan struct{})}
	session := newUpgradeStreamSession(writer, 1)

	session.RecordChunk(UpgradeStreamClientToServer, []byte("one"))
	session.RecordChunk(UpgradeStreamClientToServer, []byte("two"))
	session.RecordChunk(UpgradeStreamServerToClient, []byte("three"))

	close(writer.allowWrites)
	if err := session.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if got := session.droppedChunks.Load(); got == 0 {
		t.Fatal("expected at least one dropped chunk")
	}
	if got := session.droppedBytes.Load(); got == 0 {
		t.Fatal("expected dropped bytes to be recorded")
	}
}

type blockingWriteCloser struct {
	allowWrites chan struct{}
	buf         bytes.Buffer
}

func (w *blockingWriteCloser) Write(p []byte) (int, error) {
	<-w.allowWrites
	return w.buf.Write(p)
}

func (w *blockingWriteCloser) Close() error {
	return nil
}

type testUpgradeStreamFrame struct {
	Direction byte
	Payload   []byte
}

func readUpgradeStreamFramesForTest(t *testing.T, path string) []testUpgradeStreamFrame {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	reader := bytes.NewReader(data)

	magic := make([]byte, 4)
	if _, err := io.ReadFull(reader, magic); err != nil {
		t.Fatalf("ReadFull(magic) failed: %v", err)
	}
	if !bytes.Equal(magic, upgradeStreamFileMagic[:]) {
		t.Fatalf("magic = %q, want %q", magic, upgradeStreamFileMagic)
	}

	var version [1]byte
	if _, err := io.ReadFull(reader, version[:]); err != nil {
		t.Fatalf("ReadFull(version) failed: %v", err)
	}
	if version[0] != 1 {
		t.Fatalf("version = %d, want 1", version[0])
	}

	var startedAt int64
	if err := binary.Read(reader, binary.BigEndian, &startedAt); err != nil {
		t.Fatalf("binary.Read(startedAt) failed: %v", err)
	}

	var sessionIDLen uint16
	if err := binary.Read(reader, binary.BigEndian, &sessionIDLen); err != nil {
		t.Fatalf("binary.Read(sessionIDLen) failed: %v", err)
	}
	if _, err := reader.Seek(int64(sessionIDLen), io.SeekCurrent); err != nil {
		t.Fatalf("Seek(sessionID) failed: %v", err)
	}

	var upgradeTypeLen uint16
	if err := binary.Read(reader, binary.BigEndian, &upgradeTypeLen); err != nil {
		t.Fatalf("binary.Read(upgradeTypeLen) failed: %v", err)
	}
	if _, err := reader.Seek(int64(upgradeTypeLen), io.SeekCurrent); err != nil {
		t.Fatalf("Seek(upgradeType) failed: %v", err)
	}

	frames := make([]testUpgradeStreamFrame, 0, 2)
	for reader.Len() > 0 {
		frameType, err := reader.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte(frameType) failed: %v", err)
		}

		switch frameType {
		case streamFrameTypeData:
			var timestamp int64
			if err := binary.Read(reader, binary.BigEndian, &timestamp); err != nil {
				t.Fatalf("binary.Read(timestamp) failed: %v", err)
			}

			direction, err := reader.ReadByte()
			if err != nil {
				t.Fatalf("ReadByte(direction) failed: %v", err)
			}

			var payloadLen uint32
			if err := binary.Read(reader, binary.BigEndian, &payloadLen); err != nil {
				t.Fatalf("binary.Read(payloadLen) failed: %v", err)
			}

			payload := make([]byte, payloadLen)
			if _, err := io.ReadFull(reader, payload); err != nil {
				t.Fatalf("ReadFull(payload) failed: %v", err)
			}

			frames = append(frames, testUpgradeStreamFrame{
				Direction: direction,
				Payload:   payload,
			})
		case streamFrameTypeSummary:
			return frames
		default:
			t.Fatalf("unexpected frame type %d", frameType)
		}
	}

	return frames
}
