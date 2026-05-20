package exedev

import (
	"bytes"
	"encoding/json"
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

var nonDNSName = regexp.MustCompile(`[^a-z0-9-]+`)

type vmInfo struct {
	Name      string
	Image     string
	Status    sandbox.Status
	CreatedAt time.Time
	Tags      []string
}

func buildNewCommand(name, image, sessionID string, env map[string]string, opts sandbox.CreateOptions) string {
	cmd := newCommand("new", "--json", "--name="+name, "--no-email")
	if image != "" {
		cmd = cmd.append("--image=" + image)
	}
	if opts.Resources.CPUCores > 0 {
		cmd = cmd.append("--cpu=" + strconv.FormatFloat(opts.Resources.CPUCores, 'f', -1, 64))
	}
	if opts.Resources.MemoryMB > 0 {
		cmd = cmd.append("--memory=" + strconv.Itoa(opts.Resources.MemoryMB) + "MB")
	}
	if opts.Resources.DiskMB > 0 {
		cmd = cmd.append("--disk=" + strconv.Itoa(opts.Resources.DiskMB) + "MB")
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		cmd = cmd.append("--env", key+"="+env[key])
	}
	cmd = cmd.append("--tag=discobot,discobot-session-" + sessionID)
	return cmd.render()
}
func vmName(prefix, sessionID string) string {
	prefix = strings.Trim(nonDNSName.ReplaceAllString(strings.ToLower(prefix), "-"), "-")
	if prefix == "" {
		prefix = "discobot"
	}
	name := strings.Trim(nonDNSName.ReplaceAllString(strings.ToLower(sessionID), "-"), "-")
	if name == "" {
		name = "session"
	}
	full := prefix + "-" + name
	if len(full) > 63 {
		full = strings.TrimRight(full[:63], "-")
	}
	return full
}

func parseVM(out []byte) vmInfo {
	vms := parseVMs(out)
	if len(vms) == 0 {
		return vmInfo{}
	}
	return vms[0]
}

func parseAPIKey(out []byte) string {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return ""
	}

	var raw any
	if err := json.Unmarshal(out, &raw); err == nil {
		if key := apiKeyFromRaw(raw); key != "" {
			return key
		}
	}

	return string(out)
}

func apiKeyFromRaw(raw any) string {
	switch value := raw.(type) {
	case map[string]any:
		if key := firstString(value, "api_key", "apiKey", "key", "token", "secret", "value"); key != "" {
			return key
		}
		for _, nestedKey := range []string{"data", "result", "apiKey", "api_key"} {
			if nested, ok := value[nestedKey]; ok {
				if key := apiKeyFromRaw(nested); key != "" {
					return key
				}
			}
		}
	case []any:
		for _, item := range value {
			if key := apiKeyFromRaw(item); key != "" {
				return key
			}
		}
	case string:
		return strings.TrimSpace(value)
	}
	return ""
}

func parseVMs(out []byte) []vmInfo {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil
	}

	var raw any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}
	items := normalizeItems(raw)
	vms := make([]vmInfo, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			vm := vmFromMap(m)
			if vm.Name != "" {
				vms = append(vms, vm)
			}
		}
	}
	return vms
}

func normalizeItems(raw any) []any {
	switch value := raw.(type) {
	case []any:
		return value
	case map[string]any:
		for _, key := range []string{"vms", "machines", "instances", "items", "data", "result"} {
			if arr, ok := value[key].([]any); ok {
				return arr
			}
		}
		return []any{value}
	default:
		return nil
	}
}

func vmFromMap(m map[string]any) vmInfo {
	status := sandbox.Status(strings.ToLower(firstString(m, "status", "state", "phase")))
	switch status {
	case "", "ready", "started", "active", "up":
		status = sandbox.StatusRunning
	case "stopping", "stopped", "down", "off", "exited":
		status = sandbox.StatusStopped
	case "failed", "error", "crashed":
		status = sandbox.StatusFailed
	case "creating", "created", "pending", "starting":
		status = sandbox.StatusCreated
	}
	return vmInfo{
		Name:      firstString(m, "name", "vm_name", "vm", "hostname", "id"),
		Image:     firstString(m, "image", "image_name"),
		Status:    status,
		CreatedAt: firstTime(m, "created_at", "createdAt", "created", "ctime"),
		Tags:      tagsFromMap(m),
	}
}

func tagsFromMap(m map[string]any) []string {
	for _, key := range []string{"tags", "tag"} {
		switch value := m[key].(type) {
		case []any:
			tags := make([]string, 0, len(value))
			for _, item := range value {
				if tag, ok := item.(string); ok && tag != "" {
					tags = append(tags, tag)
				}
			}
			return tags
		case []string:
			return append([]string(nil), value...)
		case string:
			fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
			tags := make([]string, 0, len(fields))
			for _, tag := range fields {
				if tag != "" {
					tags = append(tags, tag)
				}
			}
			return tags
		}
	}
	return nil
}

func sessionIDFromTags(tags []string) string {
	for _, tag := range tags {
		if sessionID, ok := strings.CutPrefix(tag, "discobot-session-"); ok && sessionID != "" {
			return sessionID
		}
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := m[key].(type) {
		case string:
			return value
		case json.Number:
			return value.String()
		case float64:
			if value == float64(int64(value)) {
				return strconv.FormatInt(int64(value), 10)
			}
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func firstTime(m map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		if s, ok := m[key].(string); ok && s != "" {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, s); err == nil {
					return t
				}
			}
		}
	}
	return time.Time{}
}

func cloneSandbox(sb *sandbox.Sandbox) *sandbox.Sandbox {
	if sb == nil {
		return nil
	}
	clone := *sb
	clone.Metadata = maps.Clone(sb.Metadata)
	clone.Env = maps.Clone(sb.Env)
	clone.Ports = append([]sandbox.AssignedPort(nil), sb.Ports...)
	return &clone
}

func cloneSandboxes(sandboxes []*sandbox.Sandbox) []*sandbox.Sandbox {
	if sandboxes == nil {
		return nil
	}
	clones := make([]*sandbox.Sandbox, len(sandboxes))
	for i, sb := range sandboxes {
		clones[i] = cloneSandbox(sb)
	}
	return clones
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func statusOr(value, fallback sandbox.Status) sandbox.Status {
	if value != "" {
		return value
	}
	return fallback
}

func (p *Provider) vmURL(name string) string {
	return "https://" + name + "." + strings.TrimPrefix(p.cfg.VMHostSuffix, ".") + "/"
}
