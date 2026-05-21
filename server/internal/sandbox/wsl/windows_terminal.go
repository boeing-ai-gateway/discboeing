//go:build windows

package wsl

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func (m *Manager) hideWindowsTerminalWSLProfiles() error {
	distroName := strings.TrimSpace(m.cfg.WSLDistroName)
	if distroName == "" {
		return nil
	}

	for _, settingsPath := range windowsTerminalSettingsPaths() {
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read Windows Terminal settings %q: %w", settingsPath, err)
		}

		updated, changed, err := hideWindowsTerminalWSLProfilesInSettings(data, distroName, m.cfg.DesktopIconPath)
		if err != nil {
			return fmt.Errorf("update Windows Terminal settings %q: %w", settingsPath, err)
		}
		if !changed {
			continue
		}
		if err := os.WriteFile(settingsPath, updated, 0644); err != nil {
			return fmt.Errorf("write Windows Terminal settings %q: %w", settingsPath, err)
		}
	}

	return nil
}

func windowsTerminalSettingsPaths() []string {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(homeDir) != "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
	}
	if localAppData == "" {
		return nil
	}

	return []string{
		filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
		filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", "settings.json"),
		filepath.Join(localAppData, "Microsoft", "Windows Terminal", "settings.json"),
	}
}

func hideWindowsTerminalWSLProfilesInSettings(data []byte, distroName string, iconPath string) ([]byte, bool, error) {
	if strings.TrimSpace(distroName) == "" {
		return data, false, nil
	}

	spans, err := jsonObjectSpans(data)
	if err != nil {
		return nil, false, err
	}

	type replacement struct {
		start int
		end   int
		value []byte
	}

	var replacements []replacement
	for _, span := range spans {
		objectData := data[span.start:span.end]
		updatedObject, changed := hideWindowsTerminalWSLProfileObject(objectData, distroName, iconPath)
		if !changed {
			continue
		}
		replacements = append(replacements, replacement{
			start: span.start,
			end:   span.end,
			value: updatedObject,
		})
	}
	if len(replacements) == 0 {
		return data, false, nil
	}

	updated := append([]byte(nil), data...)
	for _, replacement := range slices.Backward(replacements) {
		updated = append(updated[:replacement.start], append(replacement.value, updated[replacement.end:]...)...)
	}
	return updated, true, nil
}

func hideWindowsTerminalWSLProfileObject(objectData []byte, distroName string, iconPath string) ([]byte, bool) {
	properties, err := topLevelJSONStringProperties(objectData)
	if err != nil {
		return objectData, false
	}
	if !strings.EqualFold(properties["name"], distroName) {
		return objectData, false
	}
	if properties["source"] != "Microsoft.WSL" && properties["source"] != "Windows.Terminal.Wsl" {
		return objectData, false
	}

	objectText := string(objectData)
	changed := false
	hiddenPattern := regexp.MustCompile(`(?m)("hidden"\s*:\s*)(true|false)`)
	if hiddenPattern.MatchString(objectText) {
		updated := hiddenPattern.ReplaceAllString(objectText, `${1}true`)
		if updated != objectText {
			objectText = updated
			changed = true
		}
	} else {
		updated := insertJSONProperty(objectText, "hidden", "true")
		if updated != objectText {
			objectText = updated
			changed = true
		}
	}

	if strings.TrimSpace(iconPath) != "" {
		updated := upsertJSONStringProperty(objectText, "icon", iconPath)
		if updated != objectText {
			objectText = updated
			changed = true
		}
	}

	return []byte(objectText), changed
}

func upsertJSONStringProperty(objectText string, propertyName string, value string) string {
	propertyPattern := regexp.MustCompile(`(?m)("` + regexp.QuoteMeta(propertyName) + `"\s*:\s*)("(?:\\.|[^"\\])*")`)
	if loc := propertyPattern.FindStringSubmatchIndex(objectText); loc != nil {
		return objectText[:loc[4]] + strconv.Quote(value) + objectText[loc[5]:]
	}
	return insertJSONProperty(objectText, propertyName, strconv.Quote(value))
}

func insertJSONProperty(objectText string, propertyName string, rawValue string) string {
	propertyIndent := "\t"
	if matches := regexp.MustCompile(`(?m)^([ \t]+)"[^"]+"\s*:`).FindStringSubmatch(objectText); len(matches) == 2 {
		propertyIndent = matches[1]
	}

	property := strconv.Quote(propertyName) + ": " + rawValue + ","
	rest := objectText[1:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
		return "{" + "\r\n" + propertyIndent + property + "\r\n" + rest
	}
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
		return "{" + "\n" + propertyIndent + property + "\n" + rest
	}

	return "{" + property + " " + strings.TrimLeft(rest, " \t")
}

func topLevelJSONStringProperties(objectData []byte) (map[string]string, error) {
	properties := make(map[string]string)
	if len(objectData) < 2 || objectData[0] != '{' {
		return properties, fmt.Errorf("object does not start with '{'")
	}

	depth := 0
	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(objectData); i++ {
		ch := objectData[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if inLineComment {
			if ch == '\n' || ch == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(objectData) && objectData[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(objectData) {
			switch objectData[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}

		switch ch {
		case '"':
			if depth == 1 {
				key, next, ok := consumeJSONStringProperty(objectData, i)
				if ok {
					properties[key] = next.value
					i = next.index
					continue
				}
			}
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}

	return properties, nil
}

type consumedJSONStringProperty struct {
	index int
	value string
}

func consumeJSONStringProperty(data []byte, start int) (string, consumedJSONStringProperty, bool) {
	key, keyEnd, ok := consumeJSONStringToken(data, start)
	if !ok {
		return "", consumedJSONStringProperty{}, false
	}
	index := skipJSONWhitespaceAndComments(data, keyEnd)
	if index >= len(data) || data[index] != ':' {
		return "", consumedJSONStringProperty{}, false
	}
	index = skipJSONWhitespaceAndComments(data, index+1)
	if index >= len(data) || data[index] != '"' {
		return "", consumedJSONStringProperty{}, false
	}
	value, valueEnd, ok := consumeJSONStringToken(data, index)
	if !ok {
		return "", consumedJSONStringProperty{}, false
	}
	return key, consumedJSONStringProperty{index: valueEnd - 1, value: value}, true
}

func consumeJSONStringToken(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '"' {
		return "", 0, false
	}

	var builder strings.Builder
	escaped := false
	for i := start + 1; i < len(data); i++ {
		ch := data[i]
		if escaped {
			builder.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return builder.String(), i + 1, true
		}
		builder.WriteByte(ch)
	}

	return "", 0, false
}

func skipJSONWhitespaceAndComments(data []byte, start int) int {
	inLineComment := false
	inBlockComment := false

	for i := start; i < len(data); i++ {
		if inLineComment {
			if data[i] == '\n' || data[i] == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if data[i] == '/' && i+1 < len(data) {
			switch data[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return i
		}
	}

	return len(data)
}

type objectSpan struct {
	start int
	end   int
}

func jsonObjectSpans(data []byte) ([]objectSpan, error) {
	var spans []objectSpan
	var stack []int

	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(data); i++ {
		ch := data[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if inLineComment {
			if ch == '\n' || ch == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(data) {
			switch data[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			stack = append(stack, i)
			continue
		}
		if ch != '}' {
			continue
		}
		if len(stack) == 0 {
			return nil, fmt.Errorf("unbalanced Windows Terminal settings object braces")
		}
		start := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		spans = append(spans, objectSpan{start: start, end: i + 1})
	}

	if inString || inBlockComment || len(stack) != 0 {
		return nil, fmt.Errorf("unterminated Windows Terminal settings content")
	}

	return spans, nil
}
