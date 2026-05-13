package controlsocket

import "encoding/json"

const (
	ChannelControl = "control"

	TypeChange           = "change"
	TypeError            = "error"
	TypeStreamOpen       = "stream.open"
	TypeStreamData       = "stream.data"
	TypeStreamCloseWrite = "stream.close_write"
	TypeStreamClose      = "stream.close"
)

// Frame is the shared sandbox agent control socket envelope. Control changes
// carry JSON in Payload. Stream frames use Channel as the stream name and carry
// optional JSON metadata in Payload plus raw bytes in Data.
type Frame struct {
	Version int             `json:"version"`
	ID      string          `json:"id,omitempty"`
	Channel string          `json:"channel"`
	Type    string          `json:"type"`
	Name    string          `json:"name,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Data    []byte          `json:"data,omitempty"`
}

func Payload(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
