package proxy

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/gorilla/websocket"

	"github.com/obot-platform/discobot/proxy/internal/cache"
	"github.com/obot-platform/discobot/proxy/internal/cert"
	"github.com/obot-platform/discobot/proxy/internal/config"
	"github.com/obot-platform/discobot/proxy/internal/filter"
	"github.com/obot-platform/discobot/proxy/internal/injector"
	"github.com/obot-platform/discobot/proxy/internal/logger"
	"github.com/obot-platform/discobot/proxy/internal/recorder"
)

// testLogger creates a test logger
func testLogger(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New(config.LoggingConfig{
		Level:  "error",
		Format: "text",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return log
}

// buildSOCKS5ConnectRequest builds a SOCKS5 connect request for a domain.
func buildSOCKS5ConnectRequest(host string, port int) []byte {
	req := make([]byte, 0, 5+len(host)+2)
	req = append(req, 0x05, 0x01, 0x00, 0x03, byte(len(host)))
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xff))
	return req
}

func TestIntegration_HTTPProxy_PlainHTTP(t *testing.T) {
	// Create a test HTTP server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "test")
		fmt.Fprintf(w, "Hello from backend! Method: %s, Path: %s", r.Method, r.URL.Path)
	}))
	defer backend.Close()

	// Create a simple goproxy server
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false

	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// Create HTTP client that uses the proxy
	proxyURL, _ := url.Parse(proxyServer.URL)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	// Make request through proxy
	resp, err := client.Get(backend.URL + "/test")
	if err != nil {
		t.Fatalf("Request through proxy failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Hello from backend") {
		t.Errorf("Unexpected response body: %s", body)
	}
}

func TestIntegration_HTTPProxy_StreamsCacheMissResponses(t *testing.T) {
	const firstChunkSize = 128 * 1024
	const firstReadSize = 32 * 1024
	firstChunk := strings.Repeat("a", firstChunkSize)
	secondChunk := strings.Repeat("b", 16*1024)

	release := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(firstChunk))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-release
		_, _ = w.Write([]byte(secondChunk))
	}))
	defer backend.Close()

	log := testLogger(t)
	defer log.Close()

	certMgr, err := cert.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	c, err := cache.New(t.TempDir(), 10*1024*1024, true, log.Zap())
	if err != nil {
		t.Fatalf("cache.New failed: %v", err)
	}
	matcher, err := cache.NewMatcher([]string{".*"}, false)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}
	rec, err := recorder.New(recorder.Config{Enabled: true, Dir: t.TempDir(), MaxBodySize: 64})
	if err != nil {
		t.Fatalf("recorder.New failed: %v", err)
	}
	defer rec.Close()

	httpProxy := NewHTTPProxy(certMgr, injector.New(), filter.New(), log, c, matcher, rec)
	proxyServer := httptest.NewServer(httpProxy.GetProxy())
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(backend.URL + "/stream")
	if err != nil {
		t.Fatalf("Request through proxy failed: %v", err)
	}
	defer resp.Body.Close()

	chunkCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, firstReadSize)
		_, err := io.ReadFull(resp.Body, buf)
		if err != nil {
			errCh <- err
			return
		}
		chunkCh <- string(buf)
	}()

	select {
	case got := <-chunkCh:
		if got != firstChunk[:firstReadSize] {
			t.Fatalf("first streamed chunk did not arrive before upstream completed")
		}
	case err := <-errCh:
		t.Fatalf("reading first streamed chunk failed: %v", err)
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for first streamed chunk before upstream completed")
	}

	close(release)
	rest, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(rest) != firstChunk[firstReadSize:]+secondChunk {
		t.Fatalf("remaining body length = %d, want %d", len(rest), len(firstChunk[firstReadSize:])+len(secondChunk))
	}

	resp2, err := client.Get(backend.URL + "/stream")
	if err != nil {
		t.Fatalf("Second request through proxy failed: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("Second ReadAll failed: %v", err)
	}
	if string(body2) != firstChunk+secondChunk {
		t.Fatalf("cached response body length = %d, want %d", len(body2), len(firstChunk)+len(secondChunk))
	}
	if resp2.Header.Get("X-Cache") != "HIT" {
		t.Fatal("expected second response to come from cache")
	}
}

func TestIntegration_HTTPProxy_WSSWebSocketUpgrade(t *testing.T) {
	serverErrCh := make(chan error, 1)
	recordingDir := t.TempDir()
	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErrCh <- err
			return
		}
		defer conn.Close()

		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			serverErrCh <- err
			return
		}
		if msgType != websocket.TextMessage {
			serverErrCh <- fmt.Errorf("unexpected message type %d", msgType)
			return
		}
		if string(msg) != "ping" {
			serverErrCh <- fmt.Errorf("unexpected message %q", msg)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte("pong")); err != nil {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}))
	defer backend.Close()

	log := testLogger(t)
	defer log.Close()

	certMgr, err := cert.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	c, err := cache.New(t.TempDir(), 10*1024*1024, false, log.Zap())
	if err != nil {
		t.Fatalf("cache.New failed: %v", err)
	}
	rec, err := recorder.New(recorder.Config{Enabled: true, Dir: recordingDir, MaxBodySize: 1024})
	if err != nil {
		t.Fatalf("recorder.New failed: %v", err)
	}
	defer rec.Close()

	httpProxy := NewHTTPProxy(certMgr, injector.New(), filter.New(), log, c, nil, rec)
	proxyServer := httptest.NewServer(httpProxy.GetProxy())
	defer proxyServer.Close()

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("Parse proxy URL failed: %v", err)
	}

	caPEM, err := os.ReadFile(certMgr.GetCACertPath())
	if err != nil {
		t.Fatalf("Read proxy CA failed: %v", err)
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		t.Fatal("AppendCertsFromPEM failed")
	}

	dialer := websocket.Dialer{
		Proxy:            http.ProxyURL(proxyURL),
		HandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs: rootCAs,
		},
	}

	wsURL := "wss" + strings.TrimPrefix(backend.URL, "https")
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Fatalf("WebSocket dial through proxy failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if msgType != websocket.TextMessage {
		t.Fatalf("unexpected message type %d", msgType)
	}
	if string(msg) != "pong" {
		t.Fatalf("unexpected websocket response %q", msg)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("conn.Close failed: %v", err)
	}

	select {
	case err := <-serverErrCh:
		if err != nil {
			t.Fatalf("backend websocket handler failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for backend websocket handler")
	}

	recordings := readRecordedEntries(t, recordingDir)
	if len(recordings) != 1 {
		t.Fatalf("len(recordings) = %d, want 1", len(recordings))
	}
	recording := recordings[0]
	if !recording.Upgrade {
		t.Fatal("expected recorded entry to be marked as upgrade")
	}
	if recording.UpgradeType != "websocket" {
		t.Fatalf("recording.UpgradeType = %q, want websocket", recording.UpgradeType)
	}
	if recording.StreamRecord == nil {
		t.Fatal("expected stream record metadata")
	}

	streamFrames := readUpgradeStreamFrames(t, filepath.Join(recordingDir, filepath.FromSlash(recording.StreamRecord.File)))
	if len(streamFrames) < 2 {
		t.Fatalf("len(streamFrames) = %d, want at least 2", len(streamFrames))
	}

	var sawClientPayload bool
	var sawServerPong bool
	for _, frame := range streamFrames {
		switch {
		case frame.Direction == 1 && len(frame.Payload) > 0:
			sawClientPayload = true
		case frame.Direction == 2 && bytes.Contains(frame.Payload, []byte("pong")):
			sawServerPong = true
		}
	}

	if !sawClientPayload {
		t.Fatal("expected at least one client-to-server upgraded frame")
	}
	if !sawServerPong {
		t.Fatal("expected at least one server-to-client frame containing pong payload")
	}
}

func TestIntegration_HTTPProxy_HeaderInjection(t *testing.T) {
	// Create a test HTTP server that echoes headers
	var receivedHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Parse backend URL to get host (without port for matching)
	backendHostPort := strings.TrimPrefix(backend.URL, "http://")
	backendHost, _, _ := net.SplitHostPort(backendHostPort)

	// Create proxy with header injection - use wildcard to match the IP
	inj := injector.New()
	inj.SetRules(config.HeadersConfig{
		backendHost: config.HeaderRule{
			Set: map[string]string{
				"Authorization": "Bearer test-token",
				"X-Custom":      "injected",
			},
		},
	})

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false
	proxy.OnRequest().DoFunc(func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		inj.Apply(req)
		return req, nil
	})

	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// Create HTTP client that uses the proxy
	proxyURL, _ := url.Parse(proxyServer.URL)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	// Make request through proxy
	resp, err := client.Get(backend.URL + "/test")
	if err != nil {
		t.Fatalf("Request through proxy failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify headers were injected
	if got := receivedHeaders.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := receivedHeaders.Get("X-Custom"); got != "injected" {
		t.Errorf("X-Custom header = %q, want %q", got, "injected")
	}
}

type recordedUpgradeFrame struct {
	Direction byte
	Payload   []byte
}

func readRecordedEntries(t *testing.T, dir string) []recorder.Entry {
	t.Helper()

	path := filepath.Join(dir, "requests-"+time.Now().UTC().Format("2006-01-02")+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	entries := make([]recorder.Entry, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry recorder.Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		entries = append(entries, entry)
	}

	return entries
}

func readUpgradeStreamFrames(t *testing.T, path string) []recordedUpgradeFrame {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}

	reader := bytes.NewReader(data)

	magic := make([]byte, 4)
	if _, err := io.ReadFull(reader, magic); err != nil {
		t.Fatalf("ReadFull(magic) failed: %v", err)
	}
	if string(magic) != "DBS1" {
		t.Fatalf("unexpected magic %q", magic)
	}

	if _, err := reader.Seek(1+8, io.SeekCurrent); err != nil {
		t.Fatalf("Seek(header prelude) failed: %v", err)
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

	frames := make([]recordedUpgradeFrame, 0, 2)
	for reader.Len() > 0 {
		frameType, err := reader.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte(frameType) failed: %v", err)
		}

		switch frameType {
		case 1:
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

			frames = append(frames, recordedUpgradeFrame{
				Direction: direction,
				Payload:   payload,
			})
		case 2:
			return frames
		default:
			t.Fatalf("unexpected frame type %d", frameType)
		}
	}

	return frames
}

func TestIntegration_HTTPProxy_HeaderAppend(t *testing.T) {
	// Create a test HTTP server that echoes headers
	var receivedHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create proxy with append rules
	inj := injector.New()
	inj.SetRules(config.HeadersConfig{
		"*": config.HeaderRule{
			Append: map[string]string{
				"X-Forwarded-For": "proxy.internal",
			},
		},
	})

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false
	proxy.OnRequest().DoFunc(func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		inj.Apply(req)
		return req, nil
	})

	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// Create HTTP client that uses the proxy
	proxyURL, _ := url.Parse(proxyServer.URL)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	// Make request through proxy
	resp, err := client.Get(backend.URL + "/test")
	if err != nil {
		t.Fatalf("Request through proxy failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify header contains appended value
	got := receivedHeaders.Get("X-Forwarded-For")
	if !strings.Contains(got, "proxy.internal") {
		t.Errorf("X-Forwarded-For header = %q, should contain 'proxy.internal'", got)
	}
}

func TestIntegration_SOCKS5Proxy_TCP(t *testing.T) {
	// Create a simple TCP echo server
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer echoListener.Close()

	echoAddr := echoListener.Addr().String()

	go func() {
		for {
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Start SOCKS5 proxy
	log := testLogger(t)
	flt := filter.New()
	socksProxy := NewSOCKSProxy(flt, log)

	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create SOCKS listener: %v", err)
	}
	defer socksListener.Close()

	proxyAddr := socksListener.Addr().String()

	go func() {
		for {
			conn, err := socksListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				socksProxy.ServeConn(c)
			}(conn)
		}
	}()

	// Connect to proxy via SOCKS5
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// SOCKS5 handshake
	_, err = conn.Write([]byte{0x05, 0x01, 0x00})
	if err != nil {
		t.Fatalf("Failed to send SOCKS5 greeting: %v", err)
	}

	// Read response
	resp := make([]byte, 2)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(conn, resp)
	if err != nil {
		t.Fatalf("Failed to read SOCKS5 response: %v", err)
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		t.Fatalf("Unexpected SOCKS5 response: %v", resp)
	}

	// Send connect request
	host, portStr, _ := net.SplitHostPort(echoAddr)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	connectReq := buildSOCKS5ConnectRequest(host, port)

	_, err = conn.Write(connectReq)
	if err != nil {
		t.Fatalf("Failed to send connect request: %v", err)
	}

	// Read connect response
	respHeader := make([]byte, 10)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(conn, respHeader)
	if err != nil {
		t.Fatalf("Failed to read connect response: %v", err)
	}
	if respHeader[1] != 0x00 {
		t.Fatalf("SOCKS5 connect failed with status: %d", respHeader[1])
	}

	// Test echo
	testData := "Hello through SOCKS5!"
	_, err = conn.Write([]byte(testData))
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	echoBuf := make([]byte, len(testData))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(conn, echoBuf)
	if err != nil {
		t.Fatalf("Failed to read echo response: %v", err)
	}

	if string(echoBuf) != testData {
		t.Errorf("Echo response = %q, want %q", echoBuf, testData)
	}
}

func TestIntegration_SOCKS5Proxy_Filter(t *testing.T) {
	// Start SOCKS5 proxy with filter
	log := testLogger(t)
	flt := filter.New()
	flt.SetEnabled(true)
	flt.SetAllowlist([]string{"allowed.example.com"}, nil)
	socksProxy := NewSOCKSProxy(flt, log)

	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create SOCKS listener: %v", err)
	}
	defer socksListener.Close()

	proxyAddr := socksListener.Addr().String()

	go func() {
		for {
			conn, err := socksListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				socksProxy.ServeConn(c)
			}(conn)
		}
	}()

	// Connect via SOCKS5
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// SOCKS5 handshake
	conn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	io.ReadFull(conn, resp)

	// Try blocked domain
	blockedHost := "blocked.example.com"
	connectReq := buildSOCKS5ConnectRequest(blockedHost, 80)

	conn.Write(connectReq)

	// Read response - should fail
	respHeader := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = io.ReadFull(conn, respHeader)
	if err != nil {
		// Connection closed is acceptable
		return
	}

	if respHeader[1] == 0x00 {
		t.Error("Expected SOCKS5 connect to fail for blocked domain")
	}
}

func TestIntegration_ProtocolDetection_SOCKS5(t *testing.T) {
	// Start multi-protocol server
	log := testLogger(t)
	flt := filter.New()
	socksProxy := NewSOCKSProxy(flt, log)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	proxyAddr := listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()

				proto, peeked, err := Detect(c)
				if err != nil {
					return
				}

				if proto == ProtocolSOCKS5 {
					socksProxy.ServeConn(peeked)
				}
			}(conn)
		}
	}()

	// Connect and send SOCKS5 greeting
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send SOCKS5 greeting
	conn.Write([]byte{0x05, 0x01, 0x00})

	// Read response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 2)
	_, err = io.ReadFull(conn, resp)
	if err != nil {
		t.Fatalf("Failed to read SOCKS5 response: %v", err)
	}

	if resp[0] != 0x05 {
		t.Errorf("Expected SOCKS5 version (0x05), got 0x%02x", resp[0])
	}
}

// TestIntegration_SSHOverSOCKS5 tests tunneling SSH-like traffic through SOCKS5
func TestIntegration_SSHOverSOCKS5(t *testing.T) {
	// Create mock SSH server
	sshListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer sshListener.Close()

	sshAddr := sshListener.Addr().String()

	go func() {
		for {
			conn, err := sshListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("SSH-2.0-TestServer\r\n"))
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Start SOCKS5 proxy
	log := testLogger(t)
	flt := filter.New()
	socksProxy := NewSOCKSProxy(flt, log)

	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create SOCKS listener: %v", err)
	}
	defer socksListener.Close()

	proxyAddr := socksListener.Addr().String()

	go func() {
		for {
			conn, err := socksListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				socksProxy.ServeConn(c)
			}(conn)
		}
	}()

	// Connect via SOCKS5
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// SOCKS5 handshake
	conn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	io.ReadFull(conn, resp)

	// Connect to SSH server
	host, portStr, _ := net.SplitHostPort(sshAddr)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	connectReq := buildSOCKS5ConnectRequest(host, port)
	conn.Write(connectReq)

	// Read SOCKS5 response
	respBuf := make([]byte, 10)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := io.ReadFull(conn, respBuf)
	if err != nil || n < 4 || respBuf[1] != 0x00 {
		t.Fatalf("SOCKS5 connect failed")
	}

	// Read SSH banner
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	bannerBuf := make([]byte, 100)
	n, err = conn.Read(bannerBuf)
	if err != nil {
		t.Fatalf("Failed to read SSH banner: %v", err)
	}

	banner := string(bannerBuf[:n])
	if !strings.HasPrefix(banner, "SSH-2.0") {
		t.Errorf("Expected SSH banner, got: %q", banner)
	}
}

// TestIntegration_MySQLOverSOCKS5 tests tunneling MySQL-like traffic through SOCKS5
func TestIntegration_MySQLOverSOCKS5(t *testing.T) {
	// Create mock MySQL server
	mysqlListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer mysqlListener.Close()

	mysqlAddr := mysqlListener.Addr().String()

	go func() {
		for {
			conn, err := mysqlListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// MySQL handshake packet
				handshake := []byte{
					0x4a, 0x00, 0x00, 0x00,
					0x0a,
					0x38, 0x2e, 0x30, 0x2e, 0x32, 0x38, 0x00,
					0x01, 0x00, 0x00, 0x00,
					0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				}
				c.Write(handshake)
				buf := make([]byte, 1024)
				c.Read(buf)
			}(conn)
		}
	}()

	// Start SOCKS5 proxy
	log := testLogger(t)
	flt := filter.New()
	socksProxy := NewSOCKSProxy(flt, log)

	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create SOCKS listener: %v", err)
	}
	defer socksListener.Close()

	proxyAddr := socksListener.Addr().String()

	go func() {
		for {
			conn, err := socksListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				socksProxy.ServeConn(c)
			}(conn)
		}
	}()

	// Connect via SOCKS5
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// SOCKS5 handshake
	conn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	io.ReadFull(conn, resp)

	// Connect to MySQL server
	host, portStr, _ := net.SplitHostPort(mysqlAddr)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	connectReq := buildSOCKS5ConnectRequest(host, port)
	conn.Write(connectReq)

	// Read SOCKS5 response
	respBuf := make([]byte, 10)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _ := conn.Read(respBuf)
	if n < 4 || respBuf[1] != 0x00 {
		t.Fatalf("SOCKS5 connect failed")
	}

	// Read MySQL handshake
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	mysqlBuf := make([]byte, 100)
	n, err = conn.Read(mysqlBuf)
	if err != nil {
		t.Fatalf("Failed to read MySQL handshake: %v", err)
	}

	// Check MySQL protocol version
	if n > 4 && mysqlBuf[4] != 0x0a {
		t.Errorf("Expected MySQL protocol version 10, got: %d", mysqlBuf[4])
	}
}
