package client

// CredentialsService covers project credentials and OAuth/device-code flows.
type CredentialsService struct{ client *Client }

// EventsService covers project SSE event streams.
type EventsService struct{ client *Client }

// FilesService covers session file browsing and mutation endpoints.
type FilesService struct{ client *Client }

// GitService covers workspace git endpoints.
type GitService struct{ client *Client }

// HooksService covers session hook state, output, and rerun endpoints.
type HooksService struct{ client *Client }

// ServicesService covers session service discovery, control, and output streaming.
type ServicesService struct{ client *Client }

// TerminalService covers terminal status, history, and WebSocket helpers.
type TerminalService struct{ client *Client }
