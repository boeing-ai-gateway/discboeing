package agentimpl

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/helperbin"
	"github.com/obot-platform/discobot/agent-go/thread"
)

const (
	readThreadEmbeddedScriptUnix     = "scripts/read-thread.sh"
	readThreadEmbeddedScriptWindows  = "scripts/read-thread.ps1"
	listThreadsEmbeddedScriptUnix    = "scripts/list-threads.sh"
	listThreadsEmbeddedScriptWindows = "scripts/list-threads.ps1"
)

//go:embed scripts/*.sh scripts/*.ps1
var embeddedScripts embed.FS

type stagedScript struct {
	SourcePath string
	TargetName string
}

func readThreadScriptPath() string {
	return helperbin.ScriptPath("read-thread")
}

func listThreadsScriptPath() string {
	return helperbin.ScriptPath("list-threads")
}

func applyPatchScriptPath() string {
	return helperbin.ScriptPath("apply_patch")
}

func applypatchScriptPath() string {
	return helperbin.ScriptPath("applypatch")
}

func readThreadScriptContent() string {
	return embeddedScriptContent(helperScriptSourcePath("read-thread"))
}

func listThreadsScriptContent() string {
	return embeddedScriptContent(helperScriptSourcePath("list-threads"))
}

func generateApplyPatchScriptContent(agentBin string) string {
	return generateApplyPatchScriptContentForOS(runtime.GOOS, agentBin)
}

func embeddedScriptContent(path string) string {
	content, err := fs.ReadFile(embeddedScripts, path)
	if err != nil {
		return ""
	}
	return string(content)
}

func (a *DefaultAgent) ensureHelperScripts() {
	a.ensureStagedScripts([]stagedScript{
		{SourcePath: helperScriptSourcePath("read-thread"), TargetName: "read-thread"},
		{SourcePath: helperScriptSourcePath("list-threads"), TargetName: "list-threads"},
	})
	agentBin, err := os.Executable()
	if err != nil || strings.TrimSpace(agentBin) == "" {
		return
	}
	a.ensureHelperScript(applyPatchScriptPath(), []byte(generateApplyPatchScriptContent(agentBin)))
	a.ensureHelperScript(applypatchScriptPath(), []byte(generateApplyPatchScriptContent(agentBin)))
	_ = os.Setenv("PATH", helperbin.PrependToPath(os.Getenv("PATH")))
}

func (a *DefaultAgent) ensureStagedScripts(scripts []stagedScript) {
	for _, script := range scripts {
		a.ensureStagedScript(script)
	}
}

func (a *DefaultAgent) ensureStagedScript(script stagedScript) {
	content, err := fs.ReadFile(embeddedScripts, script.SourcePath)
	if err != nil {
		return
	}
	a.ensureHelperScript(helperbin.ScriptPath(script.TargetName), content)
}

func (a *DefaultAgent) ensureHelperScript(path string, content []byte) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(content) {
		return
	}
	if err != nil && !os.IsNotExist(err) {
		return
	}

	_ = thread.WriteFileAtomic(path, content, 0o755)
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func powershellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func helperScriptSourcePath(targetName string) string {
	if runtime.GOOS == "windows" {
		switch targetName {
		case "read-thread":
			return readThreadEmbeddedScriptWindows
		case "list-threads":
			return listThreadsEmbeddedScriptWindows
		}
	}
	switch targetName {
	case "read-thread":
		return readThreadEmbeddedScriptUnix
	case "list-threads":
		return listThreadsEmbeddedScriptUnix
	default:
		return ""
	}
}

func generateApplyPatchScriptContentForOS(goos, agentBin string) string {
	if goos != "windows" {
		return "#!/usr/bin/env bash\nset -eu\n\nexec " + shellSingleQuote(agentBin) + " --discobot-run-as-apply-patch \"$@\"\n"
	}
	return strings.Join([]string{
		"param(",
		"    [Parameter(ValueFromRemainingArguments = $true)]",
		"    [string[]]$Arguments",
		")",
		"",
		"& " + powershellSingleQuote(agentBin) + " --discobot-run-as-apply-patch @Arguments",
		"if ($null -ne $LASTEXITCODE) {",
		"    exit $LASTEXITCODE",
		"}",
		"if ($?) {",
		"    exit 0",
		"}",
		"exit 1",
		"",
	}, "\n")
}
