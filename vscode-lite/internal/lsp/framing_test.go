package lsp

import (
	"bufio"
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	framed := FramePayload(payload)
	got, err := ReadFrame(bufio.NewReader(bytes.NewReader(framed)))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("expected %q, got %q", payload, got)
	}
}

func TestReadFrameRequiresContentLength(t *testing.T) {
	_, err := ReadFrame(bufio.NewReader(bytes.NewBufferString("X-Test: value\r\n\r\n{}")))
	if err == nil {
		t.Fatal("expected missing Content-Length error")
	}
}
