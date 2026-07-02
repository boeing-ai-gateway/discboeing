package frontmatter

import (
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
)

type Document[T any] struct {
	Metadata    T
	Body        string
	HasMetadata bool
}

func ParseMarkdown[T any](content string) (Document[T], error) {
	fm, body, err := parseFrontmatter(content)
	if err != nil {
		return Document[T]{}, err
	}
	metadata, err := decodeMetadata[T](fm)
	if err != nil {
		return Document[T]{}, err
	}
	return Document[T]{
		Metadata:    metadata,
		Body:        body,
		HasMetadata: fm != nil,
	}, nil
}

func ParseScript[T any](content string) (Document[T], error) {
	fm, _, err := parseScriptFrontmatterDocument(content)
	if err != nil {
		return Document[T]{}, err
	}
	metadata, err := decodeMetadata[T](fm)
	if err != nil {
		return Document[T]{}, err
	}
	return Document[T]{
		Metadata:    metadata,
		Body:        content,
		HasMetadata: fm != nil,
	}, nil
}

func parseFrontmatter(content string) (map[string]any, string, error) {
	trimmed := strings.TrimLeft(content, "\n\r")
	if !strings.HasPrefix(trimmed, "---") {
		return nil, content, nil
	}

	rest := trimmed[3:]
	if idx := strings.IndexByte(rest, '\n'); idx >= 0 {
		rest = rest[idx+1:]
	} else {
		return nil, content, nil
	}

	if before, after, ok := strings.Cut(rest, "\n---"); ok {
		yamlContent := before
		body := after
		if len(body) > 0 && body[0] == '\n' {
			body = body[1:]
		} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
			body = body[2:]
		}

		var fm map[string]any
		if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
			return nil, content, err
		}
		return fm, body, nil
	}
	return nil, content, nil
}

func decodeMetadata[T any](fm map[string]any) (T, error) {
	var metadata T
	if fm == nil {
		return metadata, nil
	}
	metadataType := reflect.TypeFor[T]()
	normalized := normalizeMetadataValue(fm, metadataType, 0)
	yamlBytes, err := yaml.Marshal(normalized)
	if err != nil {
		return metadata, fmt.Errorf("marshal normalized metadata: %w", err)
	}
	if err := yaml.Unmarshal(yamlBytes, &metadata); err != nil {
		return metadata, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return metadata, nil
}

func normalizeMetadataValue(value any, targetType reflect.Type, depth int) any {
	if targetType == nil {
		return value
	}
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType == reflect.TypeFor[providers.SupportingModels]() {
		return normalizeSupportingModelsValue(value)
	}
	switch targetType.Kind() {
	case reflect.Struct:
		input, ok := metadataMap(value)
		if !ok {
			return value
		}
		fieldIndex := metadataFieldIndex(targetType)
		output := make(map[string]any)
		for rawKey, rawValue := range input {
			field, ok := fieldIndex[normalizeMetadataKey(rawKey)]
			if !ok {
				continue
			}
			output[field.key] = normalizeMetadataFieldValue(rawValue, field.typ, depth)
		}
		return output
	case reflect.Slice:
		items, ok := value.([]any)
		if !ok {
			return value
		}
		output := make([]any, 0, len(items))
		for _, item := range items {
			output = append(output, normalizeMetadataValue(item, targetType.Elem(), depth+1))
		}
		return output
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if text, ok := value.(string); ok {
			parsed, err := strconv.Atoi(strings.TrimSpace(text))
			if err == nil {
				return parsed
			}
		}
		return value
	default:
		return value
	}
}

func normalizeMetadataFieldValue(value any, fieldType reflect.Type, depth int) any {
	trimmedType := fieldType
	for trimmedType.Kind() == reflect.Pointer {
		trimmedType = trimmedType.Elem()
	}
	if depth == 0 && trimmedType.Kind() == reflect.String {
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return normalizeMetadataValue(value, fieldType, depth+1)
}

func normalizeSupportingModelsValue(value any) any {
	switch typed := value.(type) {
	case string:
		models, err := parseSupportingModelsString(typed)
		if err == nil {
			return models
		}
		return value
	case map[string]any:
		models := make(providers.SupportingModels, len(typed))
		for key, raw := range typed {
			text, ok := raw.(string)
			if !ok {
				continue
			}
			models[providers.SupportingModelType(strings.TrimSpace(key))] = strings.TrimSpace(text)
		}
		return models
	default:
		return value
	}
}

type metadataField struct {
	key string
	typ reflect.Type
}

func metadataFieldIndex(targetType reflect.Type) map[string]metadataField {
	fields := make(map[string]metadataField)
	for _, field := range reflect.VisibleFields(targetType) {
		if !field.IsExported() {
			continue
		}
		if field.Anonymous {
			maps.Copy(fields, metadataFieldIndex(field.Type))
			continue
		}
		key := metadataFieldKey(field)
		if key == "-" || key == "" {
			continue
		}
		entry := metadataField{key: key, typ: field.Type}
		fields[normalizeMetadataKey(key)] = entry
		fields[normalizeMetadataKey(field.Name)] = entry
	}
	return fields
}

func metadataFieldKey(field reflect.StructField) string {
	tag := field.Tag.Get("yaml")
	if tag == "" {
		return field.Name
	}
	key := strings.Split(tag, ",")[0]
	if key == "" {
		return field.Name
	}
	return key
}

func metadataMap(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	if typed, ok := value.(map[string]any); ok {
		return typed, true
	}
	if typed, ok := value.(map[any]any); ok {
		output := make(map[string]any, len(typed))
		for key, raw := range typed {
			output[fmt.Sprint(key)] = raw
		}
		return output, true
	}
	return nil, false
}

func normalizeMetadataKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + ('a' - 'A'))
		}
	}
	return b.String()
}

func parseScriptFrontmatterDocument(content string) (map[string]any, string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return nil, content, nil
	}

	startLine := 0
	if strings.HasPrefix(lines[0], "#!") {
		startLine = 1
	}
	if len(lines) <= startLine {
		return nil, content, nil
	}

	delim := detectScriptDelimiter(lines[startLine])
	if delim == nil {
		return nil, content, nil
	}

	var yamlLines []string
	closeLine := -1
	for i := startLine + 1; i < len(lines); i++ {
		if matchesScriptDelimiter(lines[i], delim.delimiter) {
			closeLine = i
			break
		}
		yamlLines = append(yamlLines, stripScriptFrontMatterPrefix(lines[i], delim.prefix))
	}
	if closeLine == -1 {
		return nil, content, nil
	}

	frontmatter := make(map[string]any)
	yamlContent := strings.Join(yamlLines, "\n")
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return nil, content, fmt.Errorf("parse script front matter: %w", err)
	}

	body := strings.Join(lines[closeLine+1:], "\n")
	return frontmatter, body, nil
}

type scriptDelimiter struct {
	prefix    string
	delimiter string
}

func detectScriptDelimiter(line string) *scriptDelimiter {
	switch strings.TrimSpace(line) {
	case "---":
		return &scriptDelimiter{delimiter: "---"}
	case "#---":
		return &scriptDelimiter{prefix: "#", delimiter: "#---"}
	case "//---":
		return &scriptDelimiter{prefix: "//", delimiter: "//---"}
	default:
		return nil
	}
}

func matchesScriptDelimiter(line, delimiter string) bool {
	return strings.TrimSpace(line) == delimiter
}

func stripScriptFrontMatterPrefix(line, prefix string) string {
	if prefix == "" {
		return line
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, prefix) {
		return line
	}
	_, after, ok := strings.Cut(line, prefix)
	if !ok {
		return line
	}
	content := after
	if len(content) > 0 && (content[0] == ' ' || content[0] == '\t') {
		content = content[1:]
	}
	return content
}

func parseSupportingModelsString(value string) (providers.SupportingModels, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	models := make(providers.SupportingModels)
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key, model, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("parse supportingModels: expected key=value, got %q", item)
		}
		key = strings.TrimSpace(key)
		model = strings.TrimSpace(model)
		if key == "" || model == "" {
			return nil, fmt.Errorf("parse supportingModels: expected non-empty key=value, got %q", item)
		}
		models[providers.SupportingModelType(key)] = model
	}
	return models, nil
}
