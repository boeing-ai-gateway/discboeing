package lsp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type LanguageServer struct {
	Language string   `json:"language"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
}

type Resolver struct {
	WorkspaceRoot string
}

func (r Resolver) ResolveAll() (map[string]LanguageServer, error) {
	servers := map[string]LanguageServer{}
	goServer, err := r.Resolve("go")
	if err != nil {
		return nil, err
	}
	servers["go"] = goServer
	tsServer, err := r.Resolve("typescript")
	if err != nil {
		return nil, err
	}
	servers["typescript"] = tsServer
	servers["javascript"] = tsServer
	return servers, nil
}

func (r Resolver) Resolve(language string) (LanguageServer, error) {
	switch language {
	case "go":
		path, err := exec.LookPath("gopls")
		if err != nil {
			return LanguageServer{}, fmt.Errorf("required language server gopls not found in PATH: %w", err)
		}
		return LanguageServer{Language: "go", Command: path}, nil
	case "typescript", "javascript":
		if _, err := exec.LookPath("node"); err != nil {
			return LanguageServer{}, fmt.Errorf("required runtime node not found in PATH: %w", err)
		}
		local := filepath.Join(r.WorkspaceRoot, "node_modules", ".bin", executableName("typescript-language-server"))
		if info, err := os.Stat(local); err == nil && !info.IsDir() {
			return LanguageServer{Language: "typescript", Command: local, Args: []string{"--stdio"}}, nil
		}
		path, err := exec.LookPath("typescript-language-server")
		if err != nil {
			return LanguageServer{}, fmt.Errorf("required language server typescript-language-server not found locally or in PATH: %w", err)
		}
		return LanguageServer{Language: "typescript", Command: path, Args: []string{"--stdio"}}, nil
	default:
		return LanguageServer{}, fmt.Errorf("unsupported language: %s", language)
	}
}

func executableName(name string) string {
	if filepath.Separator == '\\' {
		return name + ".cmd"
	}
	return name
}
