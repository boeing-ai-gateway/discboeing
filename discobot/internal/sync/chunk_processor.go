package sync

import (
	"encoding/json"
	"fmt"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"
)

func applyChunk(messages []serverapi.Message, chunk serverapi.MessageChunk) []serverapi.Message {
	processor := chunkProcessor{messages: cloneMessagesForUpdate(messages)}
	processor.active = processor.lastAssistantMessage()
	processor.apply(chunk)
	return processor.messages
}

type chunkProcessor struct {
	messages []serverapi.Message
	active   *serverapi.Message
}

func (p *chunkProcessor) apply(chunk serverapi.MessageChunk) {
	switch chunk := chunk.(type) {
	case agentmessage.UserMessageChunk:
		p.messages = upsertMessage(p.messages, chunk.Data.Message, chunk.Data.InsertBeforeMessageID)
	case agentmessage.ThreadResumeChunk:
		if chunk.Data.MessageID != "" {
			p.active = p.ensureAssistantMessage(chunk.Data.MessageID, false)
		}
	case agentmessage.StartChunk:
		if chunk.MessageID != "" {
			p.active = p.ensureAssistantMessage(chunk.MessageID, true)
			mergeMessageMetadata(p.active, chunk.MessageMetadata)
		}
	case agentmessage.MessageMetadataChunk:
		if p.active != nil {
			mergeMessageMetadata(p.active, chunk.MessageMetadata)
		}
	case agentmessage.TextStartChunk:
		if p.active != nil {
			getOrCreateStreamingTextPartIndex(p.active)
		}
	case agentmessage.TextDeltaChunk:
		if p.active != nil {
			index := getOrCreateStreamingTextPartIndex(p.active)
			part := p.active.Parts[index].(agentmessage.UITextPart)
			part.Text += chunk.Delta
			p.active.Parts[index] = part
		}
	case agentmessage.TextEndChunk:
		if p.active != nil {
			finishStreamingTextPart(p.active)
		}
	case agentmessage.ReasoningStartChunk:
		if p.active != nil {
			getOrCreateStreamingReasoningPartIndex(p.active)
		}
	case agentmessage.ReasoningDeltaChunk:
		if p.active != nil {
			index := getOrCreateStreamingReasoningPartIndex(p.active)
			part := p.active.Parts[index].(agentmessage.UIReasoningPart)
			part.Text += chunk.Delta
			p.active.Parts[index] = part
		}
	case agentmessage.ReasoningEndChunk:
		if p.active != nil {
			finishStreamingReasoningPart(p.active)
		}
	case agentmessage.ToolInputStartChunk:
		if p.active != nil {
			updateToolPart(p.active, chunk.ToolCallID, chunk.ToolName, chunk.Title, func(part *agentmessage.DynamicToolPart) {
				part.State = "input-streaming"
				part.Input = nil
			})
		}
	case agentmessage.ToolInputDeltaChunk:
		if p.active != nil {
			part := findToolPart(p.active, chunk.ToolCallID)
			input := toolInputText(part) + chunk.InputTextDelta
			updateToolPart(p.active, chunk.ToolCallID, toolName(part), toolTitle(part), func(part *agentmessage.DynamicToolPart) {
				part.State = "input-streaming"
				part.Input = parsePartialToolInput(input)
			})
		}
	case agentmessage.ToolInputAvailableChunk:
		if p.active != nil {
			updateToolPart(p.active, chunk.ToolCallID, chunk.ToolName, chunk.Title, func(part *agentmessage.DynamicToolPart) {
				part.State = "input-available"
				part.Input = cloneRawJSON(chunk.Input)
			})
		}
	case agentmessage.ToolInputErrorChunk:
		if p.active != nil {
			updateToolPart(p.active, chunk.ToolCallID, chunk.ToolName, chunk.Title, func(part *agentmessage.DynamicToolPart) {
				part.State = "output-error"
				part.Input = cloneRawJSON(chunk.Input)
				part.ErrorText = chunk.ErrorText
			})
		}
	case agentmessage.ToolApprovalRequestChunk:
		if p.active != nil {
			part := findToolPart(p.active, chunk.ToolCallID)
			updateToolPart(p.active, chunk.ToolCallID, toolName(part), toolTitle(part), func(part *agentmessage.DynamicToolPart) {
				part.State = "approval-requested"
				part.Approval = &agentmessage.ToolApproval{ID: chunk.ApprovalID}
			})
			closeAssistantMessage(p.active)
			p.active = nil
		}
	case agentmessage.ToolApprovalResponseChunk:
		if p.active != nil {
			updateToolApproval(p.active, chunk.ToolCallID, chunk.ApprovalID, chunk.Approved, chunk.Reason)
		}
	case agentmessage.ToolOutputAvailableChunk:
		if p.active != nil {
			part := findToolPart(p.active, chunk.ToolCallID)
			updateToolPart(p.active, chunk.ToolCallID, toolName(part), toolTitle(part), func(part *agentmessage.DynamicToolPart) {
				part.State = "output-available"
				part.Output = cloneRawJSON(chunk.Output)
			})
		}
	case agentmessage.ToolOutputErrorChunk:
		if p.active != nil {
			part := findToolPart(p.active, chunk.ToolCallID)
			updateToolPart(p.active, chunk.ToolCallID, toolName(part), toolTitle(part), func(part *agentmessage.DynamicToolPart) {
				part.State = "output-error"
				part.ErrorText = chunk.ErrorText
			})
		}
	case agentmessage.ToolOutputDeniedChunk:
		if p.active != nil {
			part := findToolPart(p.active, chunk.ToolCallID)
			updateToolPart(p.active, chunk.ToolCallID, toolName(part), toolTitle(part), func(part *agentmessage.DynamicToolPart) {
				part.State = "output-denied"
				if part.Approval != nil && part.Approval.Approved == nil {
					part.Approval.Approved = new(false)
				}
			})
		}
	case agentmessage.ToolApprovalResponseDataChunk:
		if p.active != nil {
			updateToolApproval(p.active, chunk.Data.ToolCallID, chunk.Data.ApprovalID, chunk.Data.Approved, chunk.Data.Reason)
		}
	case agentmessage.SourceChunk:
		if p.active != nil {
			if chunk.SourceType == "document" {
				p.active.Parts = append(p.active.Parts, agentmessage.UISourceDocumentPart{Type: "source-document", SourceID: chunk.SourceID, MediaType: chunk.MediaType, Title: chunk.Title, Filename: chunk.Filename, ProviderMetadata: chunk.ProviderMetadata})
			} else {
				p.active.Parts = append(p.active.Parts, agentmessage.UISourceURLPart{Type: "source-url", SourceID: chunk.SourceID, URL: chunk.URL, Title: chunk.Title, ProviderMetadata: chunk.ProviderMetadata})
			}
		}
	case agentmessage.FileChunk:
		if p.active != nil {
			p.active.Parts = append(p.active.Parts, agentmessage.UIFilePart{Type: "file", URL: chunk.Data, MediaType: chunk.MediaType, ProviderMetadata: chunk.ProviderMetadata})
		}
	case agentmessage.StartStepChunk:
		if p.active != nil {
			p.active.Parts = append(p.active.Parts, agentmessage.UIStepStartPart{Type: "step-start"})
		}
	case agentmessage.ResponseFinishChunk, agentmessage.AbortChunk:
		if p.active != nil {
			closeAssistantMessage(p.active)
			p.active = nil
		}
	}
}

func updateToolApproval(message *serverapi.Message, toolCallID, approvalID string, approved bool, reason string) {
	for i := range message.Parts {
		part, ok := message.Parts[i].(agentmessage.DynamicToolPart)
		if !ok {
			continue
		}
		if toolCallID != "" && part.ToolCallID != toolCallID {
			continue
		}
		if toolCallID == "" && (part.Approval == nil || part.Approval.ID != approvalID) {
			continue
		}
		if part.Approval == nil {
			part.Approval = &agentmessage.ToolApproval{ID: approvalID}
		}
		part.Approval.Approved = &approved
		part.Approval.Reason = reason
		message.Parts[i] = part
		return
	}
}

func (p *chunkProcessor) ensureAssistantMessage(messageID string, reset bool) *serverapi.Message {
	for i := range p.messages {
		if p.messages[i].ID == messageID {
			if p.messages[i].Role == "assistant" && !reset {
				return &p.messages[i]
			}
			p.messages[i] = serverapi.Message{ID: messageID, Role: "assistant", Parts: []agentmessage.UIPart{}}
			return &p.messages[i]
		}
	}
	p.messages = append(p.messages, serverapi.Message{ID: messageID, Role: "assistant", Parts: []agentmessage.UIPart{}})
	return &p.messages[len(p.messages)-1]
}

func (p *chunkProcessor) lastAssistantMessage() *serverapi.Message {
	for i := len(p.messages) - 1; i >= 0; i-- {
		if p.messages[i].Role == "assistant" {
			return &p.messages[i]
		}
	}
	return nil
}

func upsertMessage(messages []serverapi.Message, message serverapi.Message, insertBeforeMessageID string) []serverapi.Message {
	for i := range messages {
		if messages[i].ID == message.ID {
			messages[i] = message
			return messages
		}
	}
	if insertBeforeMessageID != "" {
		for i := range messages {
			if messages[i].ID == insertBeforeMessageID {
				messages = append(messages, serverapi.Message{})
				copy(messages[i+1:], messages[i:])
				messages[i] = message
				return messages
			}
		}
	}
	return append(messages, message)
}

func getOrCreateStreamingTextPartIndex(message *serverapi.Message) int {
	for i := len(message.Parts) - 1; i >= 0; i-- {
		if part, ok := message.Parts[i].(agentmessage.UITextPart); ok && part.State == "streaming" {
			return i
		}
	}
	message.Parts = append(message.Parts, agentmessage.UITextPart{Type: "text", State: "streaming"})
	return len(message.Parts) - 1
}

func getOrCreateStreamingReasoningPartIndex(message *serverapi.Message) int {
	for i := len(message.Parts) - 1; i >= 0; i-- {
		if part, ok := message.Parts[i].(agentmessage.UIReasoningPart); ok && part.State == "streaming" {
			return i
		}
	}
	message.Parts = append(message.Parts, agentmessage.UIReasoningPart{Type: "reasoning", State: "streaming"})
	return len(message.Parts) - 1
}

func finishStreamingTextPart(message *serverapi.Message) {
	for i := len(message.Parts) - 1; i >= 0; i-- {
		if part, ok := message.Parts[i].(agentmessage.UITextPart); ok && part.State == "streaming" {
			part.State = "done"
			message.Parts[i] = part
			return
		}
	}
}

func finishStreamingReasoningPart(message *serverapi.Message) {
	for i := len(message.Parts) - 1; i >= 0; i-- {
		if part, ok := message.Parts[i].(agentmessage.UIReasoningPart); ok && part.State == "streaming" {
			part.State = "done"
			message.Parts[i] = part
			return
		}
	}
}

func findToolPart(message *serverapi.Message, toolCallID string) *agentmessage.DynamicToolPart {
	for i := range message.Parts {
		if part, ok := message.Parts[i].(agentmessage.DynamicToolPart); ok && part.ToolCallID == toolCallID {
			return &part
		}
	}
	return nil
}

func updateToolPart(message *serverapi.Message, toolCallID, toolNameValue, title string, update func(*agentmessage.DynamicToolPart)) {
	for i := range message.Parts {
		if part, ok := message.Parts[i].(agentmessage.DynamicToolPart); ok && part.ToolCallID == toolCallID {
			if toolNameValue != "" {
				part.ToolName = toolNameValue
			}
			if title != "" {
				part.Title = title
			}
			update(&part)
			message.Parts[i] = part
			return
		}
	}
	part := agentmessage.DynamicToolPart{Type: "dynamic-tool", ToolCallID: toolCallID, ToolName: "Unknown", State: "input-streaming"}
	if toolNameValue != "" {
		part.ToolName = toolNameValue
	}
	if title != "" {
		part.Title = title
	}
	update(&part)
	message.Parts = append(message.Parts, part)
}

func closeAssistantMessage(message *serverapi.Message) {
	for i := range message.Parts {
		switch part := message.Parts[i].(type) {
		case agentmessage.UITextPart:
			if part.State == "streaming" {
				part.State = "done"
				message.Parts[i] = part
			}
		case agentmessage.UIReasoningPart:
			if part.State == "streaming" {
				part.State = "done"
				message.Parts[i] = part
			}
		}
	}
}

func mergeMessageMetadata(message *serverapi.Message, metadata json.RawMessage) {
	if len(metadata) == 0 || string(metadata) == "null" {
		return
	}
	var next map[string]json.RawMessage
	if json.Unmarshal(metadata, &next) != nil {
		return
	}
	current := map[string]json.RawMessage{}
	if len(message.Metadata) > 0 {
		_ = json.Unmarshal(message.Metadata, &current)
	}
	for key, value := range next {
		current[key] = value
	}
	data, err := json.Marshal(current)
	if err == nil {
		message.Metadata = data
	}
}

func toolInputText(part *agentmessage.DynamicToolPart) string {
	if part == nil || len(part.Input) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(part.Input, &text) == nil {
		return text
	}
	return string(part.Input)
}

func parsePartialToolInput(text string) json.RawMessage {
	var value any
	if json.Unmarshal([]byte(text), &value) == nil {
		data, err := json.Marshal(value)
		if err == nil {
			return data
		}
	}
	data, err := json.Marshal(text)
	if err != nil {
		return json.RawMessage(fmt.Sprintf("%q", text))
	}
	return data
}

func toolName(part *agentmessage.DynamicToolPart) string {
	if part == nil {
		return ""
	}
	return part.ToolName
}

func toolTitle(part *agentmessage.DynamicToolPart) string {
	if part == nil {
		return ""
	}
	return part.Title
}

func cloneRawJSON(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}

func cloneMessagesForUpdate(messages []serverapi.Message) []serverapi.Message {
	if messages == nil {
		return nil
	}
	clone := make([]serverapi.Message, len(messages))
	for index, message := range messages {
		data, err := json.Marshal(message)
		if err != nil {
			clone[index] = message
			continue
		}
		if err := json.Unmarshal(data, &clone[index]); err != nil {
			clone[index] = message
		}
	}
	return clone
}
