package agentimpl

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/helperbin"
	"github.com/obot-platform/discobot/agent-go/thread"
)

const (
	readThreadEmbeddedScript  = "scripts/read-thread.sh"
	listThreadsEmbeddedScript = "scripts/list-threads.sh"
)

//go:embed scripts/*.sh
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
	return embeddedScriptContent(readThreadEmbeddedScript)
}

func listThreadsScriptContent() string {
	return embeddedScriptContent(listThreadsEmbeddedScript)
}

func generateApplyPatchScriptContent(agentBin string) string {
	return "#!/usr/bin/env bash\nset -eu\n\nexec " + shellSingleQuote(agentBin) + " --discobot-run-as-apply-patch \"$@\"\n"
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
		{SourcePath: readThreadEmbeddedScript, TargetName: "read-thread"},
		{SourcePath: listThreadsEmbeddedScript, TargetName: "list-threads"},
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
