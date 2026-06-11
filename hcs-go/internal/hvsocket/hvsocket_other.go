//go:build !windows

package hvsocket

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Server struct{}
type TCPProxy struct{}

func StartServer(uuid.UUID, int, bool, context.Context) (*Server, error) {
	return nil, fmt.Errorf("Hyper-V sockets are only available on Windows")
}

func (s *Server) Close() error { return nil }

func StartTCPProxy(uuid.UUID, uuid.UUID, string, int, context.Context) (*TCPProxy, error) {
	return nil, fmt.Errorf("Hyper-V sockets are only available on Windows")
}

func (p *TCPProxy) Close() error { return nil }
