package sessionconfig

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPServerConfig represents a single MCP server definition from .mcp.json.
type MCPServerConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "stdio", "sse", "http"
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	OAuth     *MCPOAuthConfig   `json:"oauth,omitempty"`
}

// MCPOAuthConfig holds OAuth settings for an HTTP/SSE MCP server.
type MCPOAuthConfig struct {
	// DynamicRegistration enables RFC 7591 dynamic client registration.
	// When true, the agent registers itself with the authorization server at runtime.
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`

	// ClientID and ClientSecret are used for pre-registered OAuth clients.
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`

	// ClientMetadataURI enables Client ID Metadata Document-based registration
	// per the MCP 2025-11-25 specification.
	ClientMetadataURI string `json:"clientMetadataUri,omitempty"`

	// Scopes is an optional list of OAuth scopes to request.
	// If empty, scopes are discovered from the authorization server metadata.
	Scopes []string `json:"scopes,omitempty"`
}

// mcpFileSchema matches the structure of a .mcp.json file.
// The top-level key "mcpServers" maps server names to their config.
type mcpFileSchema struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

// mcpServerEntry is a single server entry within .mcp.json.
type mcpServerEntry struct {
	// Stdio transport fields.
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`

	// HTTP/SSE transport fields.
	URL string `json:"url,omitempty"`

	// Shared fields.
	Type  string            `json:"type,omitempty"` // "stdio" (default), "sse", "http"
	Env   map[string]string `json:"env,omitempty"`
	OAuth *MCPOAuthConfig   `json:"oauth,omitempty"`
}

// MCPDiscoveryState captures the current discovered MCP config plus a cheap
// reload token derived from the source file contents.
type MCPDiscoveryState struct {
	Servers     []MCPServerConfig
	ReloadToken string
	ProjectRoot string
	SourceFiles []string
}

// discoverMCPServers loads MCP server definitions from the configured MCP files.
func discoverMCPServers(projectRoot string) ([]MCPServerConfig, error) {
	state, err := DiscoverMCPState(projectRoot)
	if err != nil {
		return nil, err
	}
	return state.Servers, nil
}

// DiscoverMCPState loads MCP server definitions and computes a reload token for
// the MCP config files relevant to the given working directory. It checks the
// project .mcp.json, ~/.claude/.mcp.json, ~/.discobot/mcp.json, and Discobot
// system mcp.json files.
func DiscoverMCPState(cwd string) (*MCPDiscoveryState, error) {
	projectRoot := findProjectRoot(cwd)
	paths, err := discoverMCPPaths(projectRoot)
	if err != nil {
		return nil, err
	}

	var servers []MCPServerConfig
	for _, path := range paths {
		s, err := parseMCPFile(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		servers = append(servers, s...)
	}

	token, err := computeMCPReloadToken(paths)
	if err != nil {
		return nil, err
	}

	return &MCPDiscoveryState{
		Servers:     servers,
		ReloadToken: token,
		ProjectRoot: projectRoot,
		SourceFiles: paths,
	}, nil
}

func discoverMCPPaths(projectRoot string) ([]string, error) {
	paths := []string{filepath.Join(projectRoot, ".mcp.json")}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".claude", ".mcp.json"),
			filepath.Join(home, ".discobot", "mcp.json"),
		)
	} else if err != nil {
		return nil, err
	}
	paths = append(paths, discobotSystemPaths("mcp.json")...)
	return paths, nil
}

func computeMCPReloadToken(paths []string) (string, error) {
	sum := sha256.New()
	for _, path := range paths {
		if _, err := sum.Write([]byte(path)); err != nil {
			return "", err
		}
		if _, err := sum.Write([]byte{0}); err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				if _, err := sum.Write([]byte("missing")); err != nil {
					return "", err
				}
				if _, err := sum.Write([]byte{0}); err != nil {
					return "", err
				}
				continue
			}
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		if _, err := sum.Write([]byte("present")); err != nil {
			return "", err
		}
		if _, err := sum.Write([]byte{0}); err != nil {
			return "", err
		}
		if _, err := sum.Write(data); err != nil {
			return "", err
		}
		if _, err := sum.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", sum.Sum(nil)), nil
}

// parseMCPFile reads and parses a single .mcp.json file.
// Returns nil if the file does not exist.
func parseMCPFile(path string) ([]MCPServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var schema mcpFileSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	var servers []MCPServerConfig
	for name, entry := range schema.MCPServers {
		transport := entry.Type
		if transport == "" {
			if entry.Command != "" {
				transport = "stdio"
			} else if entry.URL != "" {
				transport = "sse"
			}
		}

		servers = append(servers, MCPServerConfig{
			Name:      name,
			Transport: transport,
			Command:   entry.Command,
			Args:      entry.Args,
			URL:       entry.URL,
			Env:       expandEnvVars(entry.Env),
			OAuth:     entry.OAuth,
		})
	}

	return servers, nil
}

// expandEnvVars replaces ${VAR} patterns in env values with os.Getenv values.
func expandEnvVars(env map[string]string) map[string]string {
	if len(env) == 0 {
		return env
	}
	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = os.Expand(v, func(key string) string {
			// os.Expand handles ${VAR} and $VAR patterns.
			// Only expand from environment, don't use the input map.
			return os.Getenv(key)
		})
	}
	return result
}
