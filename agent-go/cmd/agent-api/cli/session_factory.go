package cli

import (
	"fmt"
	"strings"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/clisession"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/config"
)

func newRemoteSession(cfg *config.Config) clisession.Session {
	if strings.TrimSpace(cfg.SecretHash) == "" {
		return nil
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	return clisession.NewRemote(baseURL, cfg.SecretHash, cfg.AgentCwd)
}
