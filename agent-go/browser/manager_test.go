package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

func TestNewManagerBuildsLoopbackURL(t *testing.T) {
	t.Parallel()

	mgr, err := NewManager("session-123", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}

	info := mgr.Info()
	if info.SessionID != "session-123" {
		t.Fatalf("expected session-123, got %q", info.SessionID)
	}
	if info.WebSocketPath != "/sessions/session-123/browser/cdp" {
		t.Fatalf("unexpected websocket path %q", info.WebSocketPath)
	}
	if info.WebSocketURL == "" {
		t.Fatal("expected websocket URL")
	}
	if !mgr.TokenMatches(info.Token) {
		t.Fatal("expected token to validate")
	}
}

func TestEnvForThread(t *testing.T) {
	t.Parallel()

	mgr, err := NewManager("session-123", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}

	env := mgr.EnvForThread("thread-1")
	if got := env[EnvBrowserThreadID]; got != "thread-1" {
		t.Fatalf("expected thread-1, got %q", got)
	}
	want := "ws://127.0.0.1:3002/sessions/session-123/browser/cdp?threadId=thread-1&token="
	if got := env[EnvCDPURL]; len(got) <= len(want) || got[:len(want)] != want {
		t.Fatalf("unexpected thread env url %q", got)
	}
	if env[EnvHarnessCDPURL] != env[EnvCDPURL] {
		t.Fatal("expected harness URL to match browser URL")
	}
	if env[EnvBrowserUseCDPWS] != env[EnvCDPURL] {
		t.Fatal("expected BU_CDP_WS to match browser URL")
	}
}

func TestChromiumArgs_HeadlessWithoutDisplay(t *testing.T) {
	t.Setenv("DISPLAY", "")

	args := chromiumArgs("/tmp/profile")
	if !containsArg(args, "--headless=new") {
		t.Fatalf("expected headless arg, got %v", args)
	}
	if containsArg(args, "--start-maximized") {
		t.Fatalf("did not expect start-maximized arg, got %v", args)
	}
}

func TestChromiumArgs_UsesDisplayWhenAvailable(t *testing.T) {
	t.Setenv("DISPLAY", ":0")

	args := chromiumArgs("/tmp/profile")
	if containsArg(args, "--headless=new") {
		t.Fatalf("did not expect headless arg, got %v", args)
	}
	if !containsArg(args, "--start-maximized") {
		t.Fatalf("expected start-maximized arg, got %v", args)
	}
}

func TestPrepareUserDataDirForLaunch_RemovesStaleDevToolsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DevToolsActivePort")
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := prepareUserDataDirForLaunch(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected DevToolsActivePort to be removed, stat err=%v", err)
	}
}

func TestReadDevToolsURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "DevToolsActivePort")
	if err := os.WriteFile(path, []byte("41235\n/devtools/browser/browser-id\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wsURL, err := readDevToolsURL(path)
	if err != nil {
		t.Fatal(err)
	}
	if wsURL != "ws://127.0.0.1:41235/devtools/browser/browser-id" {
		t.Fatalf("unexpected websocket URL %q", wsURL)
	}
}

func TestResolveChromiumPathReportsMissingExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := resolveChromiumPath()
	if !errors.Is(err, ErrChromiumNotFound) {
		t.Fatalf("expected ErrChromiumNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "install one of chromium") {
		t.Fatalf("expected install guidance, got %v", err)
	}
}

func TestConfigureCDPConnAllowsLargeMessages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		_, payload, err := conn.Read(r.Context())
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			t.Errorf("unmarshal request: %v", err)
			return
		}
		resp, err := json.Marshal(map[string]any{
			"id": req.ID,
			"result": map[string]any{
				"targetInfos": []map[string]any{{
					"targetId": "page-1",
					"type":     "page",
					"url":      "https://example.com/" + strings.Repeat("x", 40000),
				}},
			},
		})
		if err != nil {
			t.Errorf("marshal response: %v", err)
			return
		}
		if err := conn.Write(r.Context(), websocket.MessageText, resp); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	ConfigureCDPConn(conn)

	client := &cdpClient{conn: conn}
	targetID, err := client.pageTargetID(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if targetID != "page-1" {
		t.Fatalf("expected page-1, got %q", targetID)
	}
}

func TestIsUniformColorPNG(t *testing.T) {
	t.Parallel()

	solid := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			solid.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var solidBuf bytes.Buffer
	if err := png.Encode(&solidBuf, solid); err != nil {
		t.Fatal(err)
	}
	uniform, err := isUniformColorPNG(solidBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !uniform {
		t.Fatal("expected solid PNG to be detected as uniform")
	}

	mixed := image.NewRGBA(image.Rect(0, 0, 2, 2))
	mixed.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	mixed.Set(1, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	mixed.Set(0, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	mixed.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var mixedBuf bytes.Buffer
	if err := png.Encode(&mixedBuf, mixed); err != nil {
		t.Fatal(err)
	}
	uniform, err = isUniformColorPNG(mixedBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if uniform {
		t.Fatal("expected non-uniform PNG to be preserved")
	}
}

func containsArg(args []string, want string) bool {
	return slices.Contains(args, want)
}
