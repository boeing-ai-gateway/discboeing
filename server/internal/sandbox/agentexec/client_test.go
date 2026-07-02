package agentexec

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

func TestAttachDemuxesFramedStreams(t *testing.T) {
	var attachQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/exec/abc/attach":
			attachQuery = r.URL.RawQuery
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Errorf("websocket accept failed: %v", err)
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "done")
			if err := conn.Write(r.Context(), websocket.MessageBinary, append([]byte{streamFrameStdout}, []byte("stdout")...)); err != nil {
				t.Errorf("write stdout frame failed: %v", err)
				return
			}
			if err := conn.Write(r.Context(), websocket.MessageBinary, append([]byte{streamFrameStderr}, []byte("stderr")...)); err != nil {
				t.Errorf("write stderr frame failed: %v", err)
				return
			}
		case r.Method == http.MethodPost && r.URL.Path == "/exec/abc/kill":
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	client := &http.Client{Transport: &testAttachTransport{
		baseURL: baseURL,
		rt:      http.DefaultTransport,
	}}
	stream, err := Attach(context.Background(), &sandbox.HTTPClientLease{Client: client}, "abc")
	if err != nil {
		t.Fatalf("Attach() failed: %v", err)
	}
	defer stream.Close()

	stdout, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll(stdout) failed: %v", err)
	}
	stderr, err := io.ReadAll(stream.Stderr())
	if err != nil {
		t.Fatalf("ReadAll(stderr) failed: %v", err)
	}
	if string(stdout) != "stdout" {
		t.Fatalf("stdout = %q, want %q", stdout, "stdout")
	}
	if string(stderr) != "stderr" {
		t.Fatalf("stderr = %q, want %q", stderr, "stderr")
	}
	if attachQuery != "" {
		t.Fatalf("attach query = %q, want empty", attachQuery)
	}
}

type testAttachTransport struct {
	baseURL *url.URL
	rt      http.RoundTripper
}

func (t *testAttachTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copied := req.Clone(req.Context())
	copied.URL = rewriteSandboxURL(t.baseURL, req.URL, "http")
	return t.rt.RoundTrip(copied)
}

func (t *testAttachTransport) WebSocketURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return rewriteSandboxURL(t.baseURL, parsed, "ws").String()
}

func rewriteSandboxURL(base, input *url.URL, scheme string) *url.URL {
	rewritten := *base
	rewritten.Scheme = strings.Replace(base.Scheme, "http", scheme, 1)
	rewritten.Path = input.Path
	rewritten.RawPath = input.RawPath
	rewritten.RawQuery = input.RawQuery
	rewritten.Fragment = input.Fragment
	return &rewritten
}
