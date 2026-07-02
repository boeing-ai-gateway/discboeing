package controlfeatures

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	agentcontrol "github.com/boeing-ai-gateway/discboeing/agent-go/internal/controlsocket"
	shared "github.com/boeing-ai-gateway/discboeing/controlsocket"
)

const gitChunkSize = 64 * 1024

type GitHTTPRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type GitHTTPResponse struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type GitTunnel struct {
	control *agentcontrol.Client
	seq     atomic.Uint64
}

func NewGitTunnel(control *agentcontrol.Client) *GitTunnel {
	return &GitTunnel{control: control}
}

func (g *GitTunnel) StartEndpoint(ctx context.Context) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	server := &http.Server{Handler: http.HandlerFunc(g.handleHTTP)}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("git tunnel endpoint failed: %v", err)
		}
	}()
	return "http://" + listener.Addr().String() + "/workspace.git", nil
}

func (g *GitTunnel) handleHTTP(w http.ResponseWriter, r *http.Request) {
	stream, err := g.control.OpenStream(r.Context(), "git:"+strconv.FormatUint(g.seq.Add(1), 10), GitHTTPRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.RawQuery,
		Headers: gitHeaders(r),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = stream.Close() }()

	bodyErr := make(chan error, 1)
	go func() {
		buf := make([]byte, gitChunkSize)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				if _, writeErr := stream.Write(buf[:n]); writeErr != nil {
					bodyErr <- writeErr
					return
				}
			}
			if err == io.EOF {
				bodyErr <- stream.CloseWrite()
				return
			}
			if err != nil {
				bodyErr <- err
				return
			}
		}
	}()

	started := false
	for {
		select {
		case err := <-bodyErr:
			if err != nil && !started {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		case frame := <-stream.Frames():
			switch frame.Type {
			case shared.TypeStreamOpen:
				var start GitHTTPResponse
				if err := json.Unmarshal(frame.Payload, &start); err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				for key, values := range start.Headers {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(start.Status)
				started = true
			case shared.TypeStreamData:
				if len(frame.Data) > 0 {
					_, _ = w.Write(frame.Data)
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				}
			case shared.TypeStreamCloseWrite, shared.TypeStreamClose:
				return
			case shared.TypeError:
				if !started {
					http.Error(w, string(frame.Payload), http.StatusBadGateway)
				}
				return
			}
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Second):
			if !started {
				http.Error(w, "timed out waiting for git response", http.StatusGatewayTimeout)
			}
			return
		}
	}
}

func gitHeaders(r *http.Request) map[string][]string {
	headers := map[string][]string{}
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Encoding", "Git-Protocol", "Accept", "User-Agent"} {
		if values := r.Header.Values(key); len(values) > 0 {
			headers[key] = values
		}
	}
	if r.ContentLength >= 0 {
		headers["Content-Length"] = []string{strconv.FormatInt(r.ContentLength, 10)}
	}
	return headers
}

func ConfigureGitRemote(ctx context.Context, workspace, remoteURL, sessionID string) {
	if workspace == "" || remoteURL == "" || sessionID == "" {
		return
	}
	_ = run(ctx, workspace, "git", "remote", "remove", "discboeing")
	if err := run(ctx, workspace, "git", "remote", "add", "discboeing", remoteURL); err != nil {
		log.Printf("configure discboeing git remote: %v", err)
		return
	}
	if err := run(ctx, workspace, "git", "config", "remote.discboeing.push", "HEAD:refs/heads/discboeing/"+sessionID); err != nil {
		log.Printf("configure discboeing push refspec: %v", err)
	}
}

func run(ctx context.Context, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
