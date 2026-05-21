package startup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

const (
	defaultPortBindRetryInterval = 10 * time.Second
	defaultPortBindTimeout       = 2 * time.Minute
)

// WaitForTCPBind blocks until addr can be bound or the timeout expires.
// It immediately releases the listener on success so regular startup can continue.
func WaitForTCPBind(ctx context.Context, addr string) error {
	return waitForTCPBind(ctx, addr, defaultPortBindTimeout, defaultPortBindRetryInterval)
}

func waitForTCPBind(ctx context.Context, addr string, timeout, retryInterval time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
		if err == nil {
			if closeErr := listener.Close(); closeErr != nil {
				return fmt.Errorf("close temporary listener for %s: %w", addr, closeErr)
			}
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("api port %s did not become available within %s (last error: %v): %w", addr, timeout, err, ctx.Err())
		}

		log.Printf("API port %s is still unavailable; retrying in %s: %v", addr, retryInterval, err)

		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return fmt.Errorf("api port %s did not become available within %s: %w", addr, timeout, ctx.Err())
		case <-timer.C:
		}
	}
}

// ListenAndServe starts server after acquiring its TCP listener. If the final
// listen races with a previous process still releasing the port, it waits and
// retries instead of surfacing an immediate fatal bind error.
func ListenAndServe(server *http.Server) error {
	return listenAndServeWithBindRetry(server, func(listener net.Listener) error {
		return server.Serve(listener)
	})
}

// ListenAndServeTLS is the TLS variant of ListenAndServe.
func ListenAndServeTLS(server *http.Server, certFile, keyFile string) error {
	return listenAndServeWithBindRetry(server, func(listener net.Listener) error {
		return server.ServeTLS(listener, certFile, keyFile)
	})
}

func listenAndServeWithBindRetry(server *http.Server, serve func(net.Listener) error) error {
	for {
		listener, err := net.Listen("tcp", server.Addr)
		if err == nil {
			return serve(listener)
		}
		if !IsAddressInUse(err) {
			return err
		}

		log.Printf("Server port %s is still unavailable; waiting before retrying listen: %v", server.Addr, err)
		if err := WaitForTCPBind(context.Background(), server.Addr); err != nil {
			return err
		}
	}
}

// IsAddressInUse reports whether err indicates a TCP address/port is already
// bound by another process. It includes string fallbacks for Windows errors that
// can arrive wrapped as localized syscall messages.
func IsAddressInUse(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EADDRINUSE) || errors.Is(err, os.ErrExist) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var syscallErr *os.SyscallError
		if errors.As(opErr.Err, &syscallErr) && errors.Is(syscallErr.Err, syscall.EADDRINUSE) {
			return true
		}
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "address already in use") ||
		strings.Contains(message, "only one usage of each socket address") ||
		strings.Contains(message, "wsaeaddrinuse")
}
