package sudoauth

import "context"

const (
	TokenCategory = "sudo"
	TokenEnvVar   = "DISCOBOT_SUDO_TOKEN"
	Guidance      = "Please use AskUserQuestion or RequestUserCredential to ask for permission to run `sudo`."
)

type AuthorizeRequest struct {
	Runtime      string            `json:"runtime"`
	Token        string            `json:"token"`
	CredentialID string            `json:"credentialId,omitempty"`
	UseID        string            `json:"useId,omitempty"`
	ToolCallID   string            `json:"toolCallId,omitempty"`
	Command      string            `json:"command,omitempty"`
	Cwd          string            `json:"cwd,omitempty"`
	Args         []string          `json:"args,omitempty"`
	PID          int               `json:"pid,omitempty"`
	PPID         int               `json:"ppid,omitempty"`
	TTY          bool              `json:"tty,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type AuthorizeResponse struct {
	Allow    bool   `json:"allow"`
	Reason   string `json:"reason,omitempty"`
	Guidance string `json:"guidance,omitempty"`
}

type Authorizer interface {
	RegisterConsoleToken(token string)
	RevokeConsoleToken(token string)
	RegisterBootstrapToken(token string)
	RevokeBootstrapToken(token string)
	AuthorizeSudo(ctx context.Context, req AuthorizeRequest) (AuthorizeResponse, error)
}
