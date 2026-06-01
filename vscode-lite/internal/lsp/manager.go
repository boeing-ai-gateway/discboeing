package lsp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"

	"github.com/coder/websocket"
)

type Manager struct {
	WorkspaceRoot string
	Servers       map[string]LanguageServer
}

func (m *Manager) Attach(ctx context.Context, language string, conn *websocket.Conn) error {
	server, ok := m.Servers[language]
	if !ok && language == "javascript" {
		server, ok = m.Servers["typescript"]
	}
	if !ok {
		return fmt.Errorf("unsupported language")
	}

	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	cmd.Dir = m.WorkspaceRoot
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	go logPipe(language, stderr)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var once sync.Once
	closeAll := func() {
		once.Do(func() {
			cancel()
			_ = stdin.Close()
			_ = conn.Close(websocket.StatusNormalClosure, "done")
		})
	}

	go func() {
		defer closeAll()
		reader := bufio.NewReader(stdout)
		for {
			payload, err := ReadFrame(reader)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}()

	for {
		messageType, payload, err := conn.Read(ctx)
		if err != nil {
			closeAll()
			return nil
		}
		if messageType != websocket.MessageText {
			continue
		}
		if err := WriteFrame(stdin, payload); err != nil {
			closeAll()
			return err
		}
	}
}

func logPipe(language string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log.Printf("lsp[%s]: %s", language, scanner.Text())
	}
}

func Accept(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
}
