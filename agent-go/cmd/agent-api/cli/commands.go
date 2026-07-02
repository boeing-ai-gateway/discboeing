package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/clisession"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
	"github.com/boeing-ai-gateway/discboeing/agent-go/sessionconfig"
)

// readMultilineInput captures input lines until endMarker is entered as a
// standalone line. It returns UI-formatted parts where pasted image file paths
// and raw image bytes are converted to file parts, and all remaining content is
// accumulated into text parts.
func readMultilineInput(prompt, endMarker, cwd string) ([]message.UIPart, error) {
	var parts []message.UIPart
	var text strings.Builder

	flushText := func() {
		if text.Len() == 0 {
			return
		}
		parts = append(parts, message.UITextPart{Text: text.String()})
		text.Reset()
	}

	for {
		line, err := readLine(prompt, nil)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) == endMarker {
			flushText()
			return parts, nil
		}

		if part, ok, err := multilinePartFromInput([]byte(line), cwd); err != nil {
			return nil, err
		} else if ok {
			flushText()
			parts = append(parts, part)
			continue
		}

		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(line)
	}
}

func multilinePartFromInput(input []byte, cwd string) (message.UIPart, bool, error) {
	if part, ok, err := imagePartFromPathInput(input, cwd); err != nil {
		return nil, false, err
	} else if ok {
		return part, true, nil
	}
	if part, ok := imagePartFromRawBytes(input); ok {
		return part, true, nil
	}
	return nil, false, nil
}

func imagePartFromPathInput(input []byte, cwd string) (message.UIFilePart, bool, error) {
	if !utf8.Valid(input) {
		return message.UIFilePart{}, false, nil
	}
	text := strings.TrimSpace(string(input))
	if text == "" {
		return message.UIFilePart{}, false, nil
	}
	text = strings.Trim(text, "\"'")
	if strings.ContainsAny(text, "\r\n\x00") {
		return message.UIFilePart{}, false, nil
	}
	path := strings.TrimPrefix(text, "file://")
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	path = filepath.Clean(path)

	fi, err := os.Stat(path)
	if err == nil && !fi.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return message.UIFilePart{}, false, err
		}
		mediaType := http.DetectContentType(data)
		if !strings.HasPrefix(mediaType, "image/") {
			extType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
			if !strings.HasPrefix(extType, "image/") {
				return message.UIFilePart{}, false, nil
			}
			mediaType = extType
		}

		return message.UIFilePart{URL: base64.StdEncoding.EncodeToString(data), MediaType: mediaType}, true, nil
	}
	return message.UIFilePart{}, false, nil
}

func imagePartFromRawBytes(input []byte) (message.UIFilePart, bool) {
	trimmed := bytes.Trim(input, "\r\n")
	if len(trimmed) == 0 {
		return message.UIFilePart{}, false
	}
	mediaType := http.DetectContentType(trimmed)
	if !strings.HasPrefix(mediaType, "image/") {
		return message.UIFilePart{}, false
	}
	return message.UIFilePart{URL: base64.StdEncoding.EncodeToString(trimmed), MediaType: mediaType}, true
}

// handleSlashCommand dispatches a slash command entered in the main input loop.
// Returns the (possibly changed) threadID and true when handled locally by the
// CLI, or current threadID and false when the command should be passed through
// to the agent.
func handleSlashCommand(ctx context.Context, line string, session clisession.Session, currentThreadID string, reg *providers.ProviderRegistry, currentModel *string, pendingFresh map[string]bool) (string, bool) {
	parts := strings.Fields(line)
	cmd := parts[0]
	switch cmd {
	case "/resume":
		return handleResumeCommand(ctx, session, currentThreadID), true
	case "/clear":
		if pendingFresh != nil {
			pendingFresh[currentThreadID] = true
		}
		fmt.Fprintln(os.Stderr, "Next message will start a fresh conversation in this thread.")
		return currentThreadID, true
	case "/models":
		handleModelsCommand(ctx, reg, currentModel)
		return currentThreadID, true
	case "/history":
		if !printThreadHistory(ctx, session, currentThreadID) {
			fmt.Fprintln(os.Stderr, "No printable messages in current thread.")
		}
		return currentThreadID, true
	default:
		if isAgentSlashCommand(ctx, session, cmd) {
			return currentThreadID, false
		}
		fmt.Fprintf(os.Stderr, "Unknown command %q. Available commands: %s\n", cmd, availableCommandsList(ctx, session))
		return currentThreadID, true
	}
}

// cliBuiltinCommands are slash commands handled directly by the CLI, not by the agent.
var cliBuiltinCommands = []string{"resume", "clear", "models", "history", "multiline"}

// availableCommands returns a sorted list of slash-prefixed command names
// available to the user (CLI built-ins + agent commands).
func availableCommands(ctx context.Context, session clisession.Session) []string {
	seen := make(map[string]struct{})
	var names []string

	add := func(name string) {
		name = "/" + strings.TrimPrefix(name, "/")
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}

	for _, name := range cliBuiltinCommands {
		add(name)
	}
	if session != nil {
		if cmds, err := session.ListCommands(ctx); err == nil {
			for _, c := range cmds {
				add(c.Name)
			}
		}
	}

	sort.Strings(names)
	return names
}

// availableCommandsList returns a sorted, slash-prefixed, comma-separated list
// of all commands available to the user (CLI built-ins + agent commands).
func availableCommandsList(ctx context.Context, session clisession.Session) string {
	return strings.Join(availableCommands(ctx, session), ", ")
}

func commandCompletionOptions(ctx context.Context, session clisession.Session) *readLineOptions {
	cmds := availableCommands(ctx, session)
	if len(cmds) == 0 {
		return nil
	}
	return &readLineOptions{slashCommands: cmds}
}

func isAgentSlashCommand(ctx context.Context, session clisession.Session, cmd string) bool {
	if session == nil {
		return false
	}
	cmds, err := session.ListCommands(ctx)
	if err != nil {
		return false
	}
	for _, c := range cmds {
		if "/"+strings.TrimPrefix(c.Name, "/") == cmd {
			return true
		}
	}
	return false
}

func agentSlashCommands(cwd string) (map[string]struct{}, error) {
	sessionCfg, err := sessionconfig.Load(cwd)
	if err != nil {
		return nil, err
	}
	commands := make(map[string]struct{}, len(sessionCfg.Skills))
	for _, skill := range sessionCfg.Skills {
		name := strings.TrimSpace(skill.Name)
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		commands[name] = struct{}{}
	}
	return commands, nil
}

// handleModelsCommand lists available models from all configured providers and
// lets the user select one to use for subsequent turns in this session.
func handleModelsCommand(ctx context.Context, reg *providers.ProviderRegistry, currentModel *string) {
	fmt.Fprint(os.Stderr, "Fetching available models...")
	spin := newSpinner()
	spin.Start()
	models, err := reg.ListModels(ctx)
	spin.Stop()
	if noColor {
		fmt.Fprintln(os.Stderr) // newline after "Fetching..." text
	} else {
		fmt.Fprint(os.Stderr, "\r\033[2K") // erase the "Fetching..." line
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching models: %v\n", err)
		return
	}
	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "No models available. Check your provider credentials.")
		return
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	// Build a list of provider entries (use provider default) followed by individual models.
	// Each entry has a selection ID (what gets stored in currentModel) and a display string.
	type entry struct {
		selectionID string // stored as the model ref
		display     string
	}
	var entries []entry

	// Provider defaults first.
	providerIDs := reg.IDs()
	for _, id := range providerIDs {
		p, err := reg.Get(id)
		if err != nil {
			continue
		}
		ref := p.DefaultModels()[providers.ModelTaskChat]
		if ref.ModelID == "" {
			continue
		}
		current := ""
		if id == *currentModel {
			current = " (current)"
		}
		entries = append(entries, entry{
			selectionID: id,
			display:     fmt.Sprintf("%s (default: %s)%s", id, ref.ModelID, current),
		})
	}

	// Individual models.
	for _, m := range models {
		current := ""
		if m.ID == *currentModel {
			current = " (current)"
		}
		display := m.ID
		if m.DisplayName != "" {
			display += " — " + m.DisplayName
		}
		entries = append(entries, entry{selectionID: m.ID, display: display + current})
	}

	fmt.Fprintln(os.Stderr, "\nAvailable models:")
	for i, e := range entries {
		fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, e.display)
	}

	for {
		input, err := readLine("\nSelect model (number, provider, or provider/model, Enter to cancel): ", nil)
		if err != nil {
			return
		}
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return
		}

		// Try as a 1-based index first.
		if n, err := strconv.Atoi(input); err == nil {
			if n < 1 || n > len(entries) {
				fmt.Fprintf(os.Stderr, "Please enter a number between 1 and %d.\n", len(entries))
				continue
			}
			*currentModel = entries[n-1].selectionID
			fmt.Fprintf(os.Stderr, "Model set to %s\n", *currentModel)
			return
		}

		// Accept a bare provider ID, a bare model relative to the current
		// provider, a supporting model type relative to the current provider,
		// or a full "provider/model" ref.
		if ref, err := reg.ResolveModelInProvider(providers.CurrentProviderFromRef(*currentModel), input, providers.ModelTaskChat); err == nil {
			if input == ref.ProviderID {
				*currentModel = input
			} else {
				*currentModel = ref.String()
			}
			fmt.Fprintf(os.Stderr, "Model set to %s\n", *currentModel)
			return
		}

		fmt.Fprintln(os.Stderr, "Enter a number from the list, a provider name, or a provider/model reference.")
	}
}
