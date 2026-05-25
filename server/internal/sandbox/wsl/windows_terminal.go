//go:build windows

package wsl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"
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

	settings, err := hujson.Parse(data)
	if err != nil {
		return nil, false, err
	}

	changed := hideWindowsTerminalWSLProfilesInValue(&settings, distroName, iconPath)
	if !changed {
		return data, false, nil
	}

	return settings.Pack(), true, nil
}

func hideWindowsTerminalWSLProfilesInValue(value *hujson.Value, distroName string, iconPath string) bool {
	changed := false

	switch trimmed := value.Value.(type) {
	case *hujson.Object:
		changed = hideWindowsTerminalWSLProfileObject(trimmed, distroName, iconPath)
		for i := range trimmed.Members {
			changed = hideWindowsTerminalWSLProfilesInValue(&trimmed.Members[i].Value, distroName, iconPath) || changed
		}
	case *hujson.Array:
		for i := range trimmed.Elements {
			changed = hideWindowsTerminalWSLProfilesInValue(&trimmed.Elements[i], distroName, iconPath) || changed
		}
	}

	return changed
}

func hideWindowsTerminalWSLProfileObject(object *hujson.Object, distroName string, iconPath string) bool {
	name, ok := hujsonObjectStringMember(object, "name")
	if !ok || !strings.EqualFold(name, distroName) {
		return false
	}

	source, ok := hujsonObjectStringMember(object, "source")
	if !ok || (source != "Microsoft.WSL" && source != "Windows.Terminal.Wsl") {
		return false
	}

	changed := upsertHUJSONBoolMember(object, "hidden", true)
	if strings.TrimSpace(iconPath) != "" && upsertHUJSONStringMember(object, "icon", iconPath) {
		changed = true
	}

	return changed
}

func hujsonObjectStringMember(object *hujson.Object, name string) (string, bool) {
	member, ok := hujsonObjectMember(object, name)
	if !ok {
		return "", false
	}
	return hujsonStringValue(&member.Value)
}

func hujsonObjectMember(object *hujson.Object, name string) (*hujson.ObjectMember, bool) {
	for i := range object.Members {
		memberName, ok := hujsonStringValue(&object.Members[i].Name)
		if !ok || memberName != name {
			continue
		}
		return &object.Members[i], true
	}
	return nil, false
}

func upsertHUJSONBoolMember(object *hujson.Object, name string, value bool) bool {
	if member, ok := hujsonObjectMember(object, name); ok {
		if current, ok := hujsonBoolValue(&member.Value); ok && current == value {
			return false
		}

		member.Value.Value = hujson.Bool(value)
		return true
	}

	appendHUJSONMember(object, name, hujson.Bool(value))
	return true
}

func upsertHUJSONStringMember(object *hujson.Object, name string, value string) bool {
	if member, ok := hujsonObjectMember(object, name); ok {
		if current, ok := hujsonStringValue(&member.Value); ok && current == value {
			return false
		}

		member.Value.Value = hujson.String(value)
		return true
	}

	appendHUJSONMember(object, name, hujson.String(value))
	return true
}

func appendHUJSONMember(object *hujson.Object, name string, value hujson.Literal) {
	var nameBefore hujson.Extra
	var nameAfter hujson.Extra
	valueBefore := hujson.Extra(" ")

	if len(object.Members) > 0 {
		lastMember := object.Members[len(object.Members)-1]
		nameBefore = cloneHUJSONExtra(lastMember.Name.BeforeExtra)
		nameAfter = cloneHUJSONExtra(lastMember.Name.AfterExtra)
		valueBefore = cloneHUJSONExtra(lastMember.Value.BeforeExtra)
	}

	object.Members = append(object.Members, hujson.ObjectMember{
		Name: hujson.Value{
			BeforeExtra: nameBefore,
			Value:       hujson.String(name),
			AfterExtra:  nameAfter,
		},
		Value: hujson.Value{
			BeforeExtra: valueBefore,
			Value:       value,
		},
	})
}

func cloneHUJSONExtra(extra hujson.Extra) hujson.Extra {
	if extra == nil {
		return nil
	}

	return append(hujson.Extra(nil), extra...)
}

func hujsonStringValue(value *hujson.Value) (string, bool) {
	literal, ok := value.Value.(hujson.Literal)
	if !ok || literal.Kind() != '"' {
		return "", false
	}

	return literal.String(), true
}

func hujsonBoolValue(value *hujson.Value) (bool, bool) {
	literal, ok := value.Value.(hujson.Literal)
	if !ok {
		return false, false
	}

	switch string(literal) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}
