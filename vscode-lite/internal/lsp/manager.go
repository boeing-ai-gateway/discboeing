package lsp

import (
	"bufio"
	"context"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
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
		return &websocket.CloseError{Code: websocket.CloseUnsupportedData, Text: "unsupported language"}
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
	closeAll := func() { once.Do(func() { cancel(); _ = stdin.Close(); _ = conn.Close() }) }

	go func() {
		defer closeAll()
		reader := bufio.NewReader(stdout)
		for {
			payload, err := ReadFrame(reader)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			closeAll()
			return nil
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

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}
