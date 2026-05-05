package agent

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	"github.com/obot-platform/discobot/agent-go/message"
)

func contentBlocks(parts []message.UIPart) ([]protocol.ContentBlock, error) {
	blocks := make([]protocol.ContentBlock, 0, len(parts))
	for _, part := range parts {
		block, err := contentBlock(part)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func contentBlock(part message.UIPart) (protocol.ContentBlock, error) {
	switch p := part.(type) {
	case message.UITextPart:
		return protocol.ContentBlockText{TextContent: protocol.TextContent{Text: p.Text}}.ContentBlock(), nil
	case message.UIReasoningPart:
		return protocol.ContentBlockText{TextContent: protocol.TextContent{Text: p.Text}}.ContentBlock(), nil
	case message.UIFilePart:
		return fileContentBlock(p)
	case message.UISourceURLPart:
		title := stringPtr(p.Title)
		return protocol.ContentBlockResourceLink{ResourceLink: protocol.ResourceLink{
			Name:  resourceName(p.Title, p.URL),
			Title: title,
			URI:   p.URL,
		}}.ContentBlock(), nil
	default:
		return protocol.ContentBlock{}, fmt.Errorf("%w: prompt part %T", errUnsupported, part)
	}
}

func fileContentBlock(part message.UIFilePart) (protocol.ContentBlock, error) {
	data, ok := dataURIContent(part.URL, part.MediaType)
	if ok {
		switch {
		case isMediaType(part.MediaType, "image"):
			return protocol.ContentBlockImage{ImageContent: protocol.ImageContent{
				Data:     data,
				MimeType: part.MediaType,
				URI:      stringPtr(part.URL),
			}}.ContentBlock(), nil
		case isMediaType(part.MediaType, "audio"):
			return protocol.ContentBlockAudio{AudioContent: protocol.AudioContent{
				Data:     data,
				MimeType: part.MediaType,
			}}.ContentBlock(), nil
		}
	}

	mimeType := stringPtr(part.MediaType)
	title := stringPtr(part.Filename)
	return protocol.ContentBlockResourceLink{ResourceLink: protocol.ResourceLink{
		MimeType: mimeType,
		Name:     resourceName(part.Filename, part.URL),
		Title:    title,
		URI:      part.URL,
	}}.ContentBlock(), nil
}

func dataURIContent(rawURL, fallbackMimeType string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "data" {
		return "", false
	}
	metadata, data, ok := strings.Cut(u.Opaque, ",")
	if !ok || !strings.Contains(metadata, ";base64") {
		return "", false
	}
	if fallbackMimeType != "" && !strings.HasPrefix(metadata, fallbackMimeType) {
		return "", false
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return "", false
	}
	return data, true
}

func isMediaType(mediaType, prefix string) bool {
	return strings.HasPrefix(mediaType, prefix+"/")
}

func resourceName(name, rawURI string) string {
	if name != "" {
		return name
	}
	u, err := url.Parse(rawURI)
	if err == nil {
		if base := path.Base(u.Path); base != "." && base != "/" {
			return base
		}
		if u.Host != "" {
			return u.Host
		}
	}
	if rawURI != "" {
		return rawURI
	}
	return "resource"
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
