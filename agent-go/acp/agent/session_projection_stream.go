package agent

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	"github.com/obot-platform/discobot/agent-go/message"
)

type sessionProjection struct {
	active []activeStreamContent

	projected []message.Message
	role      string
	parts     []message.Part

	toolNames map[protocol.ToolCallID]string
}

type activeStreamContent struct {
	id   string
	kind streamContentKind
}

type streamContentKind string

const (
	streamContentText      streamContentKind = "text"
	streamContentReasoning streamContentKind = "reasoning"
)

func newSessionProjection() *sessionProjection {
	return &sessionProjection{
		toolNames: make(map[protocol.ToolCallID]string),
	}
}

func (p *sessionProjection) push(update protocol.SessionUpdate) ([]message.MessageChunk, error) {
	switch update := update.Variant().(type) {
	case protocol.SessionUpdateUserMessageChunk:
		p.appendContent("user", update.Content, false)
		return p.closeMissing(nil), nil
	case protocol.SessionUpdateAgentMessageChunk:
		p.appendContent("assistant", update.Content, false)
		return p.contentChunks(protocol.SessionUpdateAgentMessageChunkSessionUpdate, update.Content, streamContentText), nil
	case protocol.SessionUpdateAgentThoughtChunk:
		p.appendContent("assistant", update.Content, true)
		return p.contentChunks(protocol.SessionUpdateAgentThoughtChunkSessionUpdate, update.Content, streamContentReasoning), nil
	case protocol.SessionUpdateToolCall:
		return p.toolCallChunks(update.ToolCall), nil
	case protocol.SessionUpdateToolCallUpdate:
		return p.toolCallUpdateChunks(update.ToolCallUpdate), nil
	default:
		return p.closeMissing(nil), nil
	}
}

func (p *sessionProjection) toolCallChunks(call protocol.ToolCall) []message.MessageChunk {
	p.appendToolCall(call)
	if call.ToolCallID == "" {
		return p.closeMissing(nil)
	}

	toolName := p.toolName(call.ToolCallID, call.Title, string(call.Kind))
	chunks := p.closeMissing(nil)
	chunks = append(chunks, message.ToolCallChunk{
		ToolCallID:       string(call.ToolCallID),
		ToolName:         toolName,
		Input:            string(call.RawInput),
		ProviderExecuted: new(true),
		Dynamic:          new(true),
	})
	if hasToolResult(call.Status, call.RawOutput, call.Content) {
		chunks = append(chunks, toolResultChunk(call.ToolCallID, toolName, call.Status == protocol.ToolCallStatusFailed, call.RawOutput, call.Content))
	}
	return chunks
}

func (p *sessionProjection) toolCallUpdateChunks(update protocol.ToolCallUpdate) []message.MessageChunk {
	p.appendToolCallUpdate(update)
	if update.ToolCallID == "" {
		return p.closeMissing(nil)
	}

	toolName := p.updatedToolName(update.ToolCallID, update.Title, update.Kind)
	chunks := p.closeMissing(nil)
	if update.Status == nil && len(update.RawOutput) == 0 && len(update.Content) == 0 {
		return chunks
	}

	isError := false
	if update.Status != nil {
		isError = *update.Status == protocol.ToolCallStatusFailed
	}
	return append(chunks, toolResultChunk(update.ToolCallID, toolName, isError, update.RawOutput, update.Content))
}

func (p *sessionProjection) contentChunks(updateType string, rawContent protocol.ContentBlock, kind streamContentKind) []message.MessageChunk {
	switch content := rawContent.Variant().(type) {
	case protocol.ContentBlockText:
		id := contentID(updateType, contentBlockID(rawContent.Raw()), contentMetaID(content.Meta))
		return p.textChunks(id, content.Text, kind)
	case protocol.ContentBlockImage:
		chunks := p.closeMissing(nil)
		return append(chunks, message.FileChunk{MediaType: content.MimeType, Data: content.Data})
	case protocol.ContentBlockAudio:
		chunks := p.closeMissing(nil)
		return append(chunks, message.FileChunk{MediaType: content.MimeType, Data: content.Data})
	case protocol.ContentBlockResourceLink:
		chunks := p.closeMissing(nil)
		title := content.Name
		if content.Title != nil && strings.TrimSpace(*content.Title) != "" {
			title = *content.Title
		}
		return append(chunks, message.SourceChunk{
			SourceType: "url",
			SourceID:   content.URI,
			URL:        content.URI,
			MediaType:  stringValue(content.MimeType),
			Title:      title,
			Filename:   content.Name,
		})
	default:
		return p.closeMissing(nil)
	}
}

func (p *sessionProjection) textChunks(id, text string, kind streamContentKind) []message.MessageChunk {
	chunks := p.closeMissing([]string{id})
	if activeKind, ok := p.activeKind(id); ok && activeKind != kind {
		chunks = append(chunks, p.closeID(id, activeKind))
		p.removeID(id)
	}
	if _, ok := p.activeKind(id); !ok {
		p.active = append(p.active, activeStreamContent{id: id, kind: kind})
		if kind == streamContentReasoning {
			chunks = append(chunks, message.ReasoningStartChunk{ID: id})
		} else {
			chunks = append(chunks, message.TextStartChunk{ID: id})
		}
	}
	if kind == streamContentReasoning {
		chunks = append(chunks, message.ReasoningDeltaChunk{ID: id, Delta: text})
	} else {
		chunks = append(chunks, message.TextDeltaChunk{ID: id, Delta: text})
	}
	return chunks
}

func contentBlockID(content json.RawMessage) string {
	var raw struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(content, &raw)
	return raw.ID
}

func contentMetaID(meta map[string]any) string {
	value, _ := meta["id"].(string)
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func toolDisplayName(title, kind string) string {
	if strings.TrimSpace(kind) != "" {
		return strings.TrimSpace(kind)
	}
	if strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	return "tool"
}

func hasToolResult(status protocol.ToolCallStatus, rawOutput json.RawMessage, content []protocol.ToolCallContent) bool {
	if len(rawOutput) > 0 || len(content) > 0 {
		return true
	}
	return status == protocol.ToolCallStatusCompleted || status == protocol.ToolCallStatusFailed
}

func toolResultChunk(toolCallID protocol.ToolCallID, toolName string, isError bool, rawOutput json.RawMessage, content []protocol.ToolCallContent) message.ToolResultChunk {
	result := rawOutput
	if len(result) == 0 {
		var err error
		result, err = json.Marshal(content)
		if err != nil {
			result = json.RawMessage("null")
		}
	}
	return message.ToolResultChunk{
		ToolCallID: string(toolCallID),
		ToolName:   toolName,
		Result:     result,
		IsError:    new(isError),
		Dynamic:    new(true),
	}
}

func (p *sessionProjection) closeAll() []message.MessageChunk {
	return p.closeMissing(nil)
}

func (p *sessionProjection) closeMissing(seen []string) []message.MessageChunk {
	var chunks []message.MessageChunk
	active := p.active[:0]
	for _, item := range p.active {
		if slices.Contains(seen, item.id) {
			active = append(active, item)
			continue
		}
		chunks = append(chunks, p.closeID(item.id, item.kind))
	}
	p.active = active
	return chunks
}

func (p *sessionProjection) closeID(id string, kind streamContentKind) message.MessageChunk {
	if kind == streamContentReasoning {
		return message.ReasoningEndChunk{ID: id}
	}
	return message.TextEndChunk{ID: id}
}

func (p *sessionProjection) activeKind(id string) (streamContentKind, bool) {
	for _, item := range p.active {
		if item.id == id {
			return item.kind, true
		}
	}
	return "", false
}

func (p *sessionProjection) removeID(id string) {
	p.active = slices.DeleteFunc(p.active, func(item activeStreamContent) bool {
		return item.id == id
	})
}

func (p *sessionProjection) toolName(toolCallID protocol.ToolCallID, title, kind string) string {
	toolName := toolDisplayName(title, kind)
	p.toolNames[toolCallID] = toolName
	return toolName
}

func (p *sessionProjection) updatedToolName(toolCallID protocol.ToolCallID, title *string, kind *protocol.ToolKind) string {
	toolName := p.toolNames[toolCallID]
	if title != nil || kind != nil {
		titleValue := ""
		if title != nil {
			titleValue = *title
		}
		kindValue := ""
		if kind != nil {
			kindValue = string(*kind)
		}
		toolName = toolDisplayName(titleValue, kindValue)
		p.toolNames[toolCallID] = toolName
	}
	if toolName == "" {
		toolName = string(toolCallID)
	}
	return toolName
}

func contentID(updateType, blockID, metaID string) string {
	switch {
	case strings.TrimSpace(blockID) != "":
		return strings.TrimSpace(blockID)
	case strings.TrimSpace(metaID) != "":
		return strings.TrimSpace(metaID)
	case updateType == protocol.SessionUpdateAgentThoughtChunkSessionUpdate:
		return "acp-agent-thought"
	default:
		return "acp-agent-message"
	}
}
