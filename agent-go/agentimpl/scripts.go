package agentimpl

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	return helperScriptPath("read-thread")
}

func listThreadsScriptPath() string {
	return helperScriptPath("list-threads")
}

func readThreadScriptContent() string {
	return embeddedScriptContent(readThreadEmbeddedScript)
}

func listThreadsScriptContent() string {
	return embeddedScriptContent(listThreadsEmbeddedScript)
}

func helperScriptPath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".discobot", "bin", name)
	}
	return filepath.Join(home, ".discobot", "bin", name)
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
	a.ensureHelperScript(helperScriptPath(script.TargetName), content)
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
