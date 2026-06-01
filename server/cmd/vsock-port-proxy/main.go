package main

import (
	"fmt"
	"os"

	"github.com/obot-platform/discobot/server/internal/sandbox/vm/vsockproxy"
)

func main() {
	if err := vsockproxy.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "discobot-vsock-port-proxy: %v\n", err)
		os.Exit(1)
	}
}
