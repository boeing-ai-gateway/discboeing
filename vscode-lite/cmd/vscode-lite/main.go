package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/boeing-ai-gateway/discboeing/vscode-lite/internal/lsp"
	"github.com/boeing-ai-gateway/discboeing/vscode-lite/internal/server"
	"github.com/boeing-ai-gateway/discboeing/vscode-lite/internal/vfs"
)

func main() {
	workspace := flag.String("workspace", "", "workspace directory to open")
	addr := flag.String("addr", ":3333", "address to listen on")
	staticDir := flag.String("static-dir", filepath.Join("vscode-lite", "web", "dist"), "frontend dist directory")
	flag.Parse()

	if *workspace == "" {
		fmt.Fprintln(os.Stderr, "--workspace is required")
		os.Exit(2)
	}

	filesystem, err := vfs.NewLocal(*workspace)
	if err != nil {
		log.Fatalf("invalid workspace: %v", err)
	}

	resolver := lsp.Resolver{WorkspaceRoot: filesystem.Root()}
	servers, err := resolver.ResolveAll()
	if err != nil {
		log.Fatalf("missing required vscode-lite dependency: %v", err)
	}

	manager := &lsp.Manager{WorkspaceRoot: filesystem.Root(), Servers: servers}
	app := server.New(*addr, filesystem, manager, *staticDir)
	if err := app.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
