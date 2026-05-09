package credentials

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

// EnvVar represents a credential mapped to an environment variable.
type EnvVar struct {
	CredentialID        string          `json:"credentialId,omitempty"`
	SessionCredentialID string          `json:"sessionCredentialId,omitempty"`
	Uses                []AuthorizedUse `json:"uses,omitempty"`
	EnvVar              string          `json:"envVar"`
	Value               string          `json:"value"`
	Provider            string          `json:"provider"`
	AuthType            string          `json:"authType"` // "api_key" or "oauth"
	AgentVisible        bool            `json:"agentVisible"`
	ConsoleVisible      bool            `json:"consoleVisible"`
	ServiceVisible      bool            `json:"serviceVisible"`
	HookVisible         bool            `json:"hookVisible"`
	ExpiresAt           *int64          `json:"expiresAt,omitempty"` // OAuth only (unix timestamp)
}

type AuthorizedUse struct {
	ID                 string `json:"id"`
	Description        string `json:"description"`
	LastUsedToolCallID string `json:"lastUsedToolCallId,omitempty"`
}

type ReportableUse struct {
	ID          string
	Description string
}

type ReportableBinding struct {
	CredentialID string
	EnvVar       string
	Uses         []ReportableUse
}

type gitConfigSetter func(key, value string) error

// Manager holds the current set of credentials received via the request header.
// Credentials are stored in memory and queried by the provider registry;
// the manager never writes to OS environment variables.
type Manager struct {
	mu    sync.RWMutex
	hash  string
	creds []EnvVar

	gitMu        sync.Mutex
	gitUserName  string
	gitUserEmail string
	setGitConfig gitConfigSetter
}

// NewManager creates a new credential Manager.
func NewManager() *Manager {
	return &Manager{setGitConfig: setGlobalGitConfig}
}

// Snapshot returns a copy of the currently applied env vars keyed by name.
func (m *Manager) Snapshot() map[string]string {
	return m.snapshot(func(cred EnvVar) bool { return cred.AgentVisible })
}

func (m *Manager) ServicesSnapshot() map[string]string {
	return m.snapshot(func(cred EnvVar) bool { return cred.ServiceVisible })
}

func (m *Manager) HooksSnapshot() map[string]string {
	return m.snapshot(func(cred EnvVar) bool { return cred.HookVisible })
}

func (m *Manager) snapshot(include func(EnvVar) bool) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]string, len(m.creds))
	for _, cred := range m.creds {
		if !include(cred) {
			continue
		}
		out[cred.EnvVar] = cred.Value
	}
	return out
}

// VisibleGet returns an agent-visible credential for the given environment variable name, or nil if not found.
func (m *Manager) VisibleGet(envVarName string) *EnvVar {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.creds {
		if m.creds[i].EnvVar == envVarName && m.creds[i].AgentVisible {
			return &m.creds[i]
		}
	}
	return nil
}

// Apply parses the credentials header, stores them in memory if changed,
// and configures git user if provided.
func (m *Manager) Apply(credentialsHeader, gitUserName, gitUserEmail string) {
	if credentialsHeader != "" {
		creds := parseHeader(credentialsHeader)
		m.update(creds)
	}

	m.applyGitUser(gitUserName, gitUserEmail)
}

// ForProvider returns all credentials for the given provider ID.
func (m *Manager) ForProvider(providerID string) []EnvVar {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []EnvVar
	for _, c := range m.creds {
		if c.Provider == providerID {
			result = append(result, c)
		}
	}
	return result
}

// Get returns the credential for the given environment variable name, or nil if not found.
func (m *Manager) Get(envVarName string) *EnvVar {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.creds {
		if m.creds[i].EnvVar == envVarName {
			return &m.creds[i]
		}
	}
	return nil
}

// parseHeader parses the X-Discobot-Credentials JSON array header.
func parseHeader(headerValue string) []EnvVar {
	if headerValue == "" {
		return nil
	}

	var raw []struct {
		CredentialID        string          `json:"credentialId,omitempty"`
		SessionCredentialID string          `json:"sessionCredentialId,omitempty"`
		Uses                []AuthorizedUse `json:"uses,omitempty"`
		EnvVar              string          `json:"envVar"`
		Value               string          `json:"value"`
		Provider            string          `json:"provider"`
		AuthType            string          `json:"authType"`
		AgentVisible        *bool           `json:"agentVisible"`
		ConsoleVisible      *bool           `json:"consoleVisible"`
		ServiceVisible      *bool           `json:"serviceVisible"`
		HookVisible         *bool           `json:"hookVisible"`
		ExpiresAt           *int64          `json:"expiresAt,omitempty"`
	}
	if err := json.Unmarshal([]byte(headerValue), &raw); err != nil {
		log.Printf("credentials: failed to parse header: %v", err)
		return nil
	}
	creds := make([]EnvVar, 0, len(raw))
	for _, entry := range raw {
		agentVisible := true
		if entry.AgentVisible != nil {
			agentVisible = *entry.AgentVisible
		}
		consoleVisible := false
		if entry.ConsoleVisible != nil {
			consoleVisible = *entry.ConsoleVisible
		}
		serviceVisible := false
		if entry.ServiceVisible != nil {
			serviceVisible = *entry.ServiceVisible
		}
		hookVisible := false
		if entry.HookVisible != nil {
			hookVisible = *entry.HookVisible
		}
		creds = append(creds, EnvVar{
			CredentialID:        entry.CredentialID,
			SessionCredentialID: entry.SessionCredentialID,
			Uses:                entry.Uses,
			EnvVar:              entry.EnvVar,
			Value:               entry.Value,
			Provider:            entry.Provider,
			AuthType:            entry.AuthType,
			AgentVisible:        agentVisible,
			ConsoleVisible:      consoleVisible,
			ServiceVisible:      serviceVisible,
			HookVisible:         hookVisible,
			ExpiresAt:           entry.ExpiresAt,
		})
	}
	return creds
}

func (m *Manager) SessionCredential(id string) *EnvVar {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.creds {
		if m.creds[i].SessionCredentialID == id {
			return &m.creds[i]
		}
	}
	return nil
}

func (m *Manager) SessionCredentialForValue(id, value string) *EnvVar {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.creds {
		if m.creds[i].SessionCredentialID == id && m.creds[i].Value == value {
			return &m.creds[i]
		}
	}
	return nil
}

// ReportableBindings returns the current agent-visible session-scoped
// credential bindings that can be safely communicated back to the LLM.
func (m *Manager) ReportableBindings() []ReportableBinding {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bindings := make([]ReportableBinding, 0, len(m.creds))
	for _, cred := range m.creds {
		credentialID := strings.TrimSpace(cred.SessionCredentialID)
		if !cred.AgentVisible || credentialID == "" {
			continue
		}
		uses := make([]ReportableUse, 0, len(cred.Uses))
		seenUses := make(map[string]struct{}, len(cred.Uses))
		for _, use := range cred.Uses {
			useID := strings.TrimSpace(use.ID)
			if useID == "" {
				continue
			}
			if _, ok := seenUses[useID]; ok {
				continue
			}
			seenUses[useID] = struct{}{}
			uses = append(uses, ReportableUse{
				ID:          useID,
				Description: strings.TrimSpace(use.Description),
			})
		}
		sort.Slice(uses, func(i, j int) bool {
			return uses[i].ID < uses[j].ID
		})
		bindings = append(bindings, ReportableBinding{
			CredentialID: credentialID,
			EnvVar:       strings.TrimSpace(cred.EnvVar),
			Uses:         uses,
		})
	}

	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].CredentialID != bindings[j].CredentialID {
			return bindings[i].CredentialID < bindings[j].CredentialID
		}
		return bindings[i].EnvVar < bindings[j].EnvVar
	})
	return bindings
}

// update checks if credentials changed and stores the new set if so.
func (m *Manager) update(creds []EnvVar) {
	newHash := computeHash(creds)

	m.mu.Lock()
	defer m.mu.Unlock()

	if newHash == m.hash {
		return
	}

	m.hash = newHash
	m.creds = creds
}

// computeHash computes a SHA-256 hash of credentials for change detection.
func computeHash(creds []EnvVar) string {
	// Sort by envVar for consistent ordering.
	sorted := make([]EnvVar, len(creds))
	copy(sorted, creds)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EnvVar < sorted[j].EnvVar
	})

	data, err := json.Marshal(sorted)
	if err != nil {
		return ""
	}

	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// applyGitUser sets git global user.name and user.email if provided.
// It serializes writes in-process and skips values that were already set.
func (m *Manager) applyGitUser(name, email string) {
	if name == "" && email == "" {
		return
	}

	m.gitMu.Lock()
	defer m.gitMu.Unlock()

	if name != "" && name != m.gitUserName {
		if err := m.setGitConfig("user.name", name); err != nil {
			log.Printf("credentials: failed to set git user.name: %v", err)
		} else {
			m.gitUserName = name
		}
	}
	if email != "" && email != m.gitUserEmail {
		if err := m.setGitConfig("user.email", email); err != nil {
			log.Printf("credentials: failed to set git user.email: %v", err)
		} else {
			m.gitUserEmail = email
		}
	}
}

func setGlobalGitConfig(key, value string) error {
	cmd := exec.Command("git", "config", "--global", key, value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			return fmt.Errorf("%w: %s", err, trimmedOutput)
		}
		return err
	}
	return nil
}
