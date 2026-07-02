package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all configuration for the agent-go process.
type Config struct {
	// Server settings (HTTP API mode)
	Port       int    // HTTP server port (DISCBOEING_PORT, default: 3002)
	SecretHash string // Legacy auth secret for API access (DISCBOEING_SECRET)
	TrustKey   string // Public key for signed API access tokens (DISCBOEING_TRUST_KEY)

	// Agent settings
	AgentCwd        string // Working directory for the agent (DISCBOEING_AGENT_CWD, default: cwd)
	Model           string // Default model in "providerId/modelId" format (DISCBOEING_MODEL)
	WorkspaceSource string // Workspace source path or git URL (WORKSPACE_SOURCE)
	WorkspaceOrigin string // Original workspace mount path (WORKSPACE_ORIGIN_PATH)
	WorkspaceType   string // Workspace source type (WORKSPACE_SOURCE_TYPE)
	WorkspaceCommit string // Workspace commit to check out (WORKSPACE_COMMIT)
	WorkspaceRef    string // Workspace target ref (WORKSPACE_TARGET_REF)

	// Storage
	DataDir    string // Root data directory (DISCBOEING_DATA_DIR, default: ~/.discboeing)
	ThreadsDir string // Thread persistence directory (DISCBOEING_THREADS_DIR)

	// Hooks
	HooksEnabled bool   // Enable file hooks (DISCBOEING_HOOKS_ENABLED)
	SessionID    string // Session ID for hooks (DISCBOEING_SESSION_ID, default: "default")

	// MCP OAuth settings
	MCPOAuthRedirectBase string // Base URL for OAuth callbacks (DISCBOEING_MCP_OAUTH_REDIRECT_BASE)
	DiscboeingServerURL    string // Discboeing server URL for posting tokens (DISCBOEING_SERVER_URL)
	DiscboeingProjectID    string // Project ID for the token POST path (DISCBOEING_PROJECT_ID)

	// Bootstrap/configure settings
	EnableGitControlSocket bool // Enable the git control socket bridge
}

// DynamicConfigRequired reports whether server mode should start in the
// lightweight bootstrap state and wait for POST /configure before serving the
// agent API.
func (c *Config) DynamicConfigRequired() bool {
	return getEnvBool("DISCBOEING_WAIT_FOR_CONFIG", false)
}

// Load reads configuration from environment variables.
// Call godotenv.Load() before this to support .env files.
func Load() *Config {
	cwd, _ := os.Getwd()

	cfg := &Config{}

	// Server
	cfg.Port = getEnvInt("DISCBOEING_PORT", 3002)
	cfg.SecretHash = getEnv("DISCBOEING_SECRET", "")
	cfg.TrustKey = getEnv("DISCBOEING_TRUST_KEY", "")

	// Agent
	cfg.AgentCwd = getEnv("DISCBOEING_AGENT_CWD", getEnv("WORKSPACE_PATH", cwd))
	cfg.Model = getEnv("DISCBOEING_MODEL", "")
	cfg.WorkspaceSource = getEnv("WORKSPACE_SOURCE", "")
	cfg.WorkspaceOrigin = getEnv("WORKSPACE_ORIGIN_PATH", "")
	cfg.WorkspaceType = getEnv("WORKSPACE_SOURCE_TYPE", getEnv("DISCBOEING_WORKSPACE_SOURCE_TYPE", ""))
	cfg.WorkspaceCommit = getEnv("WORKSPACE_COMMIT", "")
	cfg.WorkspaceRef = getEnv("WORKSPACE_TARGET_REF", "")

	// Storage — default to ~/.discboeing
	home, _ := os.UserHomeDir()
	if home == "" {
		home = cwd
	}
	cfg.DataDir = getEnv("DISCBOEING_DATA_DIR", filepath.Join(home, ".discboeing"))
	cfg.ThreadsDir = getEnv("DISCBOEING_THREADS_DIR", filepath.Join(cfg.DataDir, "threads"))

	// Hooks
	cfg.HooksEnabled = getEnvBool("DISCBOEING_HOOKS_ENABLED", false)
	cfg.SessionID = getEnv("DISCBOEING_SESSION_ID", "default")

	// MCP OAuth
	cfg.MCPOAuthRedirectBase = getEnv("DISCBOEING_MCP_OAUTH_REDIRECT_BASE", "")
	cfg.DiscboeingServerURL = getEnv("DISCBOEING_SERVER_URL", "")
	cfg.DiscboeingProjectID = getEnv("DISCBOEING_PROJECT_ID", "")
	cfg.EnableGitControlSocket = getEnvBool("DISCBOEING_ENABLE_GIT_CONTROL_SOCKET", false)

	return cfg
}

// ProviderConfig holds configuration for a single LLM provider.
type ProviderConfig struct {
	APIKey  string
	BaseURL string
}

// ProviderConfigs reads provider configuration from environment variables.
// For each provider ID, it checks {UPPER_ID}_API_KEY and optionally {UPPER_ID}_BASE_URL.
// Returns a map of provider ID → ProviderConfig for providers that have an API key set.
func ProviderConfigs(providerIDs []string) map[string]ProviderConfig {
	configs := make(map[string]ProviderConfig)
	for _, id := range providerIDs {
		prefix := strings.ToUpper(id) + "_"
		apiKey := os.Getenv(prefix + "API_KEY")
		if apiKey == "" {
			continue
		}
		pc := ProviderConfig{APIKey: apiKey}
		if baseURL := os.Getenv(prefix + "BASE_URL"); baseURL != "" {
			pc.BaseURL = baseURL
		}
		configs[id] = pc
	}
	return configs
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
