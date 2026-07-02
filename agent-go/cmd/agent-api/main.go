// Command agent-api runs the agent API.
//
// By default it runs as an interactive terminal agent (stdin/stdout).
// Pass --server to start the HTTP API server instead.
//
// Configuration is entirely via environment variables (see internal/config).
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/boeing-ai-gateway/discboeing/agent-go/cmd/agent-api/cli"
	"github.com/boeing-ai-gateway/discboeing/agent-go/cmd/agent-api/server"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/config"
	"github.com/boeing-ai-gateway/discboeing/agent-go/tools"

	// Side-effect imports register provider factories so the registry can
	// build them on demand when credentials arrive via X-Discboeing-Credentials.
	_ "github.com/boeing-ai-gateway/discboeing/agent-go/providers/anthropic"
	_ "github.com/boeing-ai-gateway/discboeing/agent-go/providers/openai"
	_ "github.com/boeing-ai-gateway/discboeing/agent-go/providers/openaicompatible"
)

const runAsApplyPatchArg = "--discboeing-run-as-apply-patch"

func main() {
	// Handle subcommands before flag parsing so they get clean args.
	if len(os.Args) >= 2 && os.Args[1] == runAsApplyPatchArg {
		os.Exit(runStandaloneApplyPatch(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	if len(os.Args) >= 2 && os.Args[1] == "login" {
		if err := cli.RunLogin(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "login: %v\n", err)
			os.Exit(1)
		}
		return
	}

	serverMode := flag.Bool("server", false, "Run as HTTP API server (default: interactive terminal mode)")
	flags := cli.AddFlags()
	flag.Parse()

	_ = godotenv.Load()
	cfg := config.Load()

	// When invoked as "discboeing-agent-api" (drop-in replacement), default to server mode.
	if filepath.Base(os.Args[0]) == "discboeing-agent-api" || *serverMode {
		server.Run(cfg)
	} else if flags.PrintMode() {
		os.Exit(cli.RunPrint(cfg, flags, flag.Args()))
	} else {
		cli.Run(cfg, flags)
	}
}

func runStandaloneApplyPatch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	patch, err := standaloneApplyPatchInput(args, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "apply_patch: %v\n", err)
		return 1
	}

	_ = godotenv.Load()
	cfg := config.Load()
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "apply_patch: determine working directory: %v\n", err)
		return 1
	}

	out, err := tools.StandaloneApplyPatch(cwd, cfg.DataDir, cfg.ThreadsDir, patch)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	_, _ = io.WriteString(stdout, out)
	if !bytes.HasSuffix([]byte(out), []byte("\n")) {
		_, _ = io.WriteString(stdout, "\n")
	}
	return 0
}

func standaloneApplyPatchInput(args []string, stdin io.Reader) (string, error) {
	switch len(args) {
	case 0:
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return "", fmt.Errorf("patch input is required")
		}
		return string(data), nil
	case 1:
		if args[0] == "" {
			return "", fmt.Errorf("patch input is required")
		}
		return args[0], nil
	default:
		return "", fmt.Errorf("expected patch input from stdin or a single argument")
	}
}
