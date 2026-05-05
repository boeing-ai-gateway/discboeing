package agent

import (
	"encoding/json"
	"strings"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	discobotagent "github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/message"
)

func (p *sessionProjection) messages() []message.Message {
	p.flush()
	return p.projected
}

func (p *sessionProjection) appendContent(role string, block protocol.ContentBlock, reasoning bool) {
	part, ok := projectedContentPart(block, reasoning)
	if !ok {
		return
	}
	p.ensureRole(role)
	p.parts = append(p.parts, part)
}

func (p *sessionProjection) appendToolCall(call protocol.ToolCall) {
	if call.ToolCallID == "" {
		return
	}
	p.ensureRole("assistant")
	toolName := p.toolName(call.ToolCallID, call.Title, string(call.Kind))
	p.parts = append(p.parts, message.ToolCallPart{
		ToolCallID:       string(call.ToolCallID),
		ToolName:         toolName,
		Input:            string(call.RawInput),
		ProviderExecuted: new(true),
	})
	if hasToolResult(call.Status, call.RawOutput, call.Content) {
		p.parts = append(p.parts, projectedToolResult(call.ToolCallID, toolName, call.Status == protocol.ToolCallStatusFailed, call.RawOutput, call.Content))
	}
}

func (p *sessionProjection) appendToolCallUpdate(update protocol.ToolCallUpdate) {
	if update.ToolCallID == "" {
		return
	}
	p.ensureRole("assistant")
	toolName := p.updatedToolName(update.ToolCallID, update.Title, update.Kind)
	if update.Status == nil && len(update.RawOutput) == 0 && len(update.Content) == 0 {
		return
	}
	isError := false
	if update.Status != nil {
		isError = *update.Status == protocol.ToolCallStatusFailed
	}
	p.parts = append(p.parts, projectedToolResult(update.ToolCallID, toolName, isError, update.RawOutput, update.Content))
}

func (p *sessionProjection) ensureRole(role string) {
	if p.role == role {
		return
	}
	p.flush()
	p.role = role
}

func (p *sessionProjection) flush() {
	if p.role == "" || len(p.parts) == 0 {
		p.role = ""
		p.parts = nil
		return
	}
	p.projected = append(p.projected, message.Message{Role: p.role, Parts: p.parts})
	p.role = ""
	p.parts = nil
}

func projectedContentPart(block protocol.ContentBlock, reasoning bool) (message.Part, bool) {
	switch content := block.Variant().(type) {
	case protocol.ContentBlockText:
		id := contentID(protocol.SessionUpdateAgentMessageChunkSessionUpdate, contentBlockID(block.Raw()), contentMetaID(content.Meta))
		if reasoning {
			id = contentID(protocol.SessionUpdateAgentThoughtChunkSessionUpdate, contentBlockID(block.Raw()), contentMetaID(content.Meta))
			return message.ReasoningPart{ID: id, Text: content.Text, State: "done"}, true
		}
		return message.TextPart{ID: id, Text: content.Text, State: "done"}, true
	case protocol.ContentBlockImage:
		data := content.Data
		if strings.TrimSpace(data) == "" && content.URI != nil {
			data = *content.URI
		}
		return message.FilePart{Data: data, MediaType: content.MimeType}, true
	case protocol.ContentBlockAudio:
		return message.FilePart{Data: content.Data, MediaType: content.MimeType}, true
	case protocol.ContentBlockResourceLink:
		title := content.Name
		if content.Title != nil && strings.TrimSpace(*content.Title) != "" {
			title = *content.Title
		}
		return message.SourceURLPart{SourceID: content.URI, URL: content.URI, Title: title}, true
	default:
		return nil, false
	}
}

func projectedToolResult(toolCallID protocol.ToolCallID, toolName string, isError bool, rawOutput json.RawMessage, content []protocol.ToolCallContent) message.ToolResultPart {
	return message.ToolResultPart{
		ToolCallID: string(toolCallID),
		ToolName:   toolName,
		Output:     projectedToolOutput(isError, rawOutput, content),
	}
}

func projectedToolOutput(isError bool, rawOutput json.RawMessage, content []protocol.ToolCallContent) message.ToolResultOutput {
	if len(rawOutput) > 0 {
		var text string
		if json.Unmarshal(rawOutput, &text) == nil {
			if isError {
				return message.ErrorTextOutput{Value: text}
			}
			return message.TextOutput{Value: text}
		}
		if json.Valid(rawOutput) {
			if isError {
				return message.ErrorJSONOutput{Value: rawOutput}
			}
			return message.JSONOutput{Value: rawOutput}
		}
	}
	text := projectedToolContentText(content)
	if isError {
		return message.ErrorTextOutput{Value: text}
	}
	return message.TextOutput{Value: text}
}

func projectedToolContentText(content []protocol.ToolCallContent) string {
	var parts []string
	for _, item := range content {
		switch value := item.Variant().(type) {
		case protocol.ToolCallContentContent:
			if part, ok := projectedContentPart(value.Content.Content, false); ok {
				switch part := part.(type) {
				case message.TextPart:
					parts = append(parts, part.Text)
				case message.FilePart:
					parts = append(parts, part.Data)
				case message.SourceURLPart:
					parts = append(parts, part.URL)
				}
			}
		case protocol.ToolCallContentDiff:
			parts = append(parts, value.Path+"\n"+value.NewText)
		case protocol.ToolCallContentTerminal:
			parts = append(parts, string(value.TerminalID))
		default:
			parts = append(parts, string(item.Raw()))
		}
	}
	return strings.Join(parts, "\n")
}

func projectedUIMessages(projected []message.Message) ([]message.UIMessage, error) {
	if len(projected) == 0 {
		return nil, nil
	}
	for i := range projected {
		if projected[i].ID == "" {
			projected[i].ID = "acp-" + discobotagent.GenerateID()
		}
	}
	return message.ProjectUIMessages(projected)
}

func (m *sessionManager) appendPromptMessages(threadID string, user message.UIMessage, projection *sessionProjection) error {
	projected, err := projectedUIMessages(projection.messages())
	if err != nil {
		return err
	}
	messages := []message.UIMessage{user}
	messages = append(messages, projected...)
	m.state.AppendMessages(threadID, messages)
	return nil
}
