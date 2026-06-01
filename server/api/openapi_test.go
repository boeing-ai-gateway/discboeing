package serverapi

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/obot-platform/discobot/agent-go/message"
)

func TestOpenAPISpecValid(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(OpenAPISpec)
	if err != nil {
		t.Fatalf("load OpenAPI spec: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI spec: %v", err)
	}
}

func TestOpenAPIMessageSchemaMatchesUIMessageJSONTags(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(OpenAPISpec)
	if err != nil {
		t.Fatalf("load OpenAPI spec: %v", err)
	}

	schema := doc.Components.Schemas["Message"]
	if schema == nil || schema.Value == nil {
		t.Fatal("OpenAPI Message schema not found")
	}

	jsonFields := uiMessageJSONFields()
	for field := range jsonFields {
		if schema.Value.Properties[field] == nil {
			t.Fatalf("UIMessage JSON field %q is missing from OpenAPI Message schema", field)
		}
	}

	for field := range schema.Value.Properties {
		if !jsonFields[field] {
			t.Fatalf("OpenAPI Message schema field %q is missing from UIMessage JSON tags", field)
		}
	}
}

func uiMessageJSONFields() map[string]bool {
	fields := map[string]bool{
		// UIMessage.Parts has json:"-" because MarshalJSON handles UIPart
		// polymorphism explicitly, but the wire format still emits "parts".
		"parts": true,
	}
	messageType := reflect.TypeFor[message.UIMessage]()
	for field := range messageType.Fields() {
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name, _, _ := strings.Cut(jsonTag, ",")
		if name == "" {
			continue
		}
		fields[name] = true
	}
	return fields
}
