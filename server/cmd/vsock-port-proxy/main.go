package main

import (
	"fmt"
	"os"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/vm/vsockproxy"
)

func main() {
	if err := vsockproxy.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "discboeing-vsock-port-proxy: %v\n", err)
		os.Exit(1)
	}
}
