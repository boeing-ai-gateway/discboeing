package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/agentimpl"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/clisession"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/config"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/credentials"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
	"github.com/boeing-ai-gateway/discboeing/agent-go/tools"
)

// RunPrint runs one prompt, writes only the final assistant text to stdout, and exits.
func RunPrint(cfg *config.Config, flags *Flags, args []string) int {
	prompt, err := printPromptInput(args, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	oauthBase, oauthSrv := startOAuthServer()
	if oauthSrv != nil {
		defer oauthSrv.Close()
	}

	credMgr := credentials.NewManager()
	reg := providers.NewProviderRegistry(credMgr)
	store := thread.NewStore(cfg.ThreadsDir)
	exec := tools.New(cfg.AgentCwd, cfg.DataDir, "")
	exec.SetThreadsDir(cfg.ThreadsDir)

	mcpCfg := agentimpl.NewMCPConfig(
		oauthBase,
		cfg.SessionID,
		cfg.DiscboeingServerURL,
		cfg.DiscboeingProjectID,
	)
	a := agentimpl.NewDefaultAgent(store, reg, exec, cfg.AgentCwd, mcpCfg)
	var session clisession.Session = clisession.NewLocal(a, store, cfg.AgentCwd)
	if remote := newRemoteSession(cfg); remote != nil {
		session = remote
	}
	defer session.Close()

	if oauthSrv != nil {
		wireOAuthCallbacks(oauthSrv, a)
	}
	go watchMCPOAuth(ctx, a)

	threadID := selectInitialThreadID(cfg, false, "")
	req := buildPrintPromptRequest(cfg, flags, prompt)

	if flags.JSONOutput() {
		if err := runPrintJSONLTurnLoop(ctx, session, threadID, req, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	out, err := runPrintTurnLoop(ctx, session, threadID, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprint(os.Stdout, out)
	if !strings.HasSuffix(out, "\n") {
		fmt.Fprintln(os.Stdout)
	}
	return 0
}

func buildPrintPromptRequest(cfg *config.Config, flags *Flags, prompt string) agent.PromptRequest {
	model := cfg.Model
	if flags != nil && flags.model != nil && *flags.model != "" {
		model = *flags.model
	}
	reasoning := ""
	if flags == nil || flags.reasoning == nil || *flags.reasoning {
		reasoning = "enabled"
	}

	return agent.PromptRequest{
		Model:        model,
		Reasoning:    reasoning,
		ServiceTier:  servicePriorityValue(flags),
		UserParts:    []message.UIPart{message.UITextPart{Text: prompt}},
		MaxTurns:     maxTurnsValue(flags),
		SubagentType: subagentValue(flags),
	}
}

func printPromptInput(args []string, stdin io.Reader) (string, error) {
	if len(args) > 0 {
		prompt := strings.TrimSpace(strings.Join(args, " "))
		if prompt == "" {
			return "", fmt.Errorf("prompt is required")
		}
		return prompt, nil
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", fmt.Errorf("prompt is required; pass it as arguments or on stdin")
	}
	return prompt, nil
}

func runPrintTurnLoop(ctx context.Context, session clisession.Session, threadID string, req agent.PromptRequest) (string, error) {
	var out strings.Builder
	toolState := newToolRenderState()
	resumeOnly := false

	for {
		var (
			seq iter.Seq2[message.MessageChunk, error]
			err error
		)
		if resumeOnly {
			seq, err = session.Resume(ctx, threadID, agent.PromptRequest{})
		} else {
			seq, err = session.Prompt(ctx, threadID, req)
		}
		if err != nil {
			return "", err
		}

		if err := collectPrintOutput(ctx, seq, &out, toolState); err != nil {
			return "", err
		}

		pending, err := session.PendingQuestion(ctx, threadID)
		if err != nil {
			return "", fmt.Errorf("checking for pending question: %w", err)
		}
		if pending == nil {
			return out.String(), nil
		}

		if !handlePendingQuestion(ctx, session, threadID, pending) {
			return "", errors.New("could not collect answer for pending question")
		}
		resumeOnly = true
		req = agent.PromptRequest{}
	}
}

func runPrintJSONLTurnLoop(ctx context.Context, session clisession.Session, threadID string, req agent.PromptRequest, out io.Writer) error {
	resumeOnly := false

	for {
		var (
			seq iter.Seq2[message.MessageChunk, error]
			err error
		)
		if resumeOnly {
			seq, err = session.Resume(ctx, threadID, agent.PromptRequest{})
		} else {
			seq, err = session.Prompt(ctx, threadID, req)
		}
		if err != nil {
			return err
		}

		if err := collectPrintJSONL(ctx, seq, out); err != nil {
			return err
		}

		pending, err := session.PendingQuestion(ctx, threadID)
		if err != nil {
			return fmt.Errorf("checking for pending question: %w", err)
		}
		if pending == nil {
			return nil
		}

		if !handlePendingQuestion(ctx, session, threadID, pending) {
			return errors.New("could not collect answer for pending question")
		}
		resumeOnly = true
		req = agent.PromptRequest{}
	}
}

func collectPrintOutput(ctx context.Context, seq iter.Seq2[message.MessageChunk, error], out *strings.Builder, toolState *toolRenderState) error {
	for chunk, err := range seq {
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
			return err
		}
		if chunk == nil {
			continue
		}
		if c, ok := chunk.(message.TextDeltaChunk); ok {
			out.WriteString(c.Delta)
			continue
		}
		renderChunk(chunk, nil, toolState)
	}
	return ctx.Err()
}

func collectPrintJSONL(ctx context.Context, seq iter.Seq2[message.MessageChunk, error], out io.Writer) error {
	for chunk, err := range seq {
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
			return err
		}
		if chunk == nil {
			continue
		}
		data, err := message.MarshalChunk(chunk)
		if err != nil {
			return err
		}
		if _, err := out.Write(data); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func maxTurnsValue(flags *Flags) int {
	if flags == nil || flags.maxTurns == nil {
		return 0
	}
	return *flags.maxTurns
}

func subagentValue(flags *Flags) string {
	if flags == nil || flags.subagent == nil {
		return ""
	}
	return *flags.subagent
}

func servicePriorityValue(flags *Flags) string {
	if flags == nil || flags.servicePriority == nil {
		return ""
	}
	return *flags.servicePriority
}
