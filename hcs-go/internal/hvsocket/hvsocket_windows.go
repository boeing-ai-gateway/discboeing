//go:build windows

package hvsocket

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"

	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/winapi"
)

const (
	afHyperV      = 34
	sockStream    = 1
	hvProtocolRaw = 1
	socketError   = ^uintptr(0)
	wsaWouldBlock = 10035
	fionbio       = 0x8004667e
)

type sockaddrHv struct {
	Family    uint16
	Reserved  uint16
	VMID      windows.GUID
	ServiceID windows.GUID
}

type Server struct {
	socket uintptr
	cancel context.CancelFunc
	done   chan struct{}
}

type TCPProxy struct {
	socket uintptr
	cancel context.CancelFunc
	done   chan struct{}
}

func StartServer(vmID uuid.UUID, port int, echo bool, parent context.Context) (*Server, error) {
	serviceID, err := PortToServiceID(port)
	if err != nil {
		return nil, err
	}
	socket, err := listenSocket(vmID, serviceID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	server := &Server{socket: socket, cancel: cancel, done: make(chan struct{})}
	go server.acceptLoop(ctx, echo)
	return server, nil
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.cancel()
	winapi.ProcCloseSocket.Call(s.socket)
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
	}
	winapi.ProcWSACleanup.Call()
	return nil
}

func StartTCPProxy(vmID uuid.UUID, serviceID uuid.UUID, tcpHost string, tcpPort int, parent context.Context) (*TCPProxy, error) {
	socket, err := listenSocket(vmID, serviceID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	proxy := &TCPProxy{socket: socket, cancel: cancel, done: make(chan struct{})}
	go proxy.acceptLoop(ctx, tcpHost, tcpPort)
	return proxy, nil
}

func (p *TCPProxy) Close() error {
	if p == nil {
		return nil
	}
	p.cancel()
	winapi.ProcCloseSocket.Call(p.socket)
	select {
	case <-p.done:
	case <-time.After(2 * time.Second):
	}
	winapi.ProcWSACleanup.Call()
	return nil
}

func listenSocket(vmID uuid.UUID, serviceID uuid.UUID) (uintptr, error) {
	var data [400]byte
	ret, _, _ := winapi.ProcWSAStartup.Call(0x202, uintptr(unsafe.Pointer(&data[0])))
	if ret != 0 {
		return 0, fmt.Errorf("WSAStartup failed: %d", ret)
	}
	socket, _, err := winapi.ProcWSASocketW.Call(afHyperV, sockStream, hvProtocolRaw, 0, 0, 0)
	if socket == socketError {
		winapi.ProcWSACleanup.Call()
		return 0, fmt.Errorf("WSASocket(AF_HYPERV) failed: %w", err)
	}
	addr := sockaddrHv{Family: afHyperV, VMID: winapi.GUIDFromUUID(vmID), ServiceID: winapi.GUIDFromUUID(serviceID)}
	ret, _, err = winapi.ProcBind.Call(socket, uintptr(unsafe.Pointer(&addr)), unsafe.Sizeof(addr))
	if ret == socketError {
		winapi.ProcCloseSocket.Call(socket)
		winapi.ProcWSACleanup.Call()
		return 0, fmt.Errorf("bind(AF_HYPERV) failed: %w", err)
	}
	ret, _, err = winapi.ProcListen.Call(socket, 8)
	if ret == socketError {
		winapi.ProcCloseSocket.Call(socket)
		winapi.ProcWSACleanup.Call()
		return 0, fmt.Errorf("listen(AF_HYPERV) failed: %w", err)
	}
	var nonblocking uint32 = 1
	ret, _, err = winapi.ProcIoctlSocket.Call(socket, fionbio, uintptr(unsafe.Pointer(&nonblocking)))
	if ret == socketError {
		winapi.ProcCloseSocket.Call(socket)
		winapi.ProcWSACleanup.Call()
		return 0, fmt.Errorf("ioctlsocket(AF_HYPERV) failed: %w", err)
	}
	return socket, nil
}

func (s *Server) acceptLoop(ctx context.Context, echo bool) {
	defer close(s.done)
	for ctx.Err() == nil {
		client, _, _ := winapi.ProcAccept.Call(s.socket, 0, 0)
		if client == socketError {
			if lastSocketError() == wsaWouldBlock {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return
		}
		go handleClient(ctx, client, echo)
	}
}

func (p *TCPProxy) acceptLoop(ctx context.Context, tcpHost string, tcpPort int) {
	defer close(p.done)
	for ctx.Err() == nil {
		client, _, _ := winapi.ProcAccept.Call(p.socket, 0, 0)
		if client == socketError {
			if lastSocketError() == wsaWouldBlock {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return
		}
		go proxyClient(ctx, client, tcpHost, tcpPort)
	}
}

func handleClient(ctx context.Context, client uintptr, echo bool) {
	defer winapi.ProcCloseSocket.Call(client)
	buffer := make([]byte, 16*1024)
	for ctx.Err() == nil {
		n, err := recv(client, buffer)
		if n <= 0 || err != nil {
			return
		}
		fmt.Printf("HVSOCK received %d byte(s): %x\n", n, buffer[:n])
		if echo {
			_ = sendAll(client, buffer[:n])
		}
	}
}

func proxyClient(ctx context.Context, client uintptr, tcpHost string, tcpPort int) {
	defer winapi.ProcCloseSocket.Call(client)
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", tcpHost, tcpPort))
	if err != nil {
		return
	}
	defer conn.Close()
	done := make(chan struct{}, 2)
	go func() { copyHvToTCP(client, conn); done <- struct{}{} }()
	go func() { copyTCPToHv(conn, client); done <- struct{}{} }()
	select {
	case <-ctx.Done():
	case <-done:
	}
}

func copyHvToTCP(client uintptr, conn net.Conn) {
	buffer := make([]byte, 64*1024)
	for {
		n, err := recv(client, buffer)
		if n <= 0 || err != nil {
			return
		}
		if _, err := conn.Write(buffer[:n]); err != nil {
			return
		}
	}
}

func copyTCPToHv(conn net.Conn, client uintptr) {
	buffer := make([]byte, 64*1024)
	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			if err := sendAll(client, buffer[:n]); err != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				return
			}
			return
		}
	}
}

func recv(socket uintptr, buffer []byte) (int, error) {
	rc, _, err := winapi.ProcRecv.Call(socket, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
	if rc == socketError {
		return 0, err
	}
	return int(rc), nil
}

func sendAll(socket uintptr, buffer []byte) error {
	for len(buffer) > 0 {
		rc, _, err := winapi.ProcSend.Call(socket, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
		if rc == socketError {
			return err
		}
		buffer = buffer[int(rc):]
	}
	return nil
}

func lastSocketError() uintptr {
	rc, _, _ := winapi.ProcWSAGetLastError.Call()
	return rc
}
