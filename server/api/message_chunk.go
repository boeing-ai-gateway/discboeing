package serverapi

import agentmessage "github.com/boeing-ai-gateway/discboeing/agent-go/message"

// UnmarshalMessageChunk decodes a discriminated chat stream chunk using the
// authoritative agent-go/message implementation.
func UnmarshalMessageChunk(data []byte) (MessageChunk, error) {
	return agentmessage.UnmarshalChunk(data)
}

// MarshalMessageChunk encodes a discriminated chat stream chunk using the
// authoritative agent-go/message implementation.
func MarshalMessageChunk(chunk MessageChunk) ([]byte, error) {
	return agentmessage.MarshalChunk(chunk)
}
