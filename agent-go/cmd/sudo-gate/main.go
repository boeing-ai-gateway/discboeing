//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	guidance       = "Please use AskUserQuestion or RequestUserCredential to ask for permission to run `sudo`."
	configPath     = "/etc/discobot/sudo-gate.json"
	defaultTimeout = 10 * time.Second
)

type gateConfig struct {
	RealSudo    string `json:"realSudo"`
	AgentAPIURL string `json:"agentAPIURL"`
}

type authorizeRequest struct {
	Runtime      string            `json:"runtime"`
	Token        string            `json:"token"`
	CredentialID string            `json:"credentialId,omitempty"`
	UseID        string            `json:"useId,omitempty"`
	ToolCallID   string            `json:"toolCallId,omitempty"`
	Command      string            `json:"command,omitempty"`
	Cwd          string            `json:"cwd,omitempty"`
	Args         []string          `json:"args,omitempty"`
	PID          int               `json:"pid,omitempty"`
	PPID         int               `json:"ppid,omitempty"`
	TTY          bool              `json:"tty,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type authorizeResponse struct {
	Allow    bool   `json:"allow"`
	Reason   string `json:"reason,omitempty"`
	Guidance string `json:"guidance,omitempty"`
}

func main() {
	cfg, err := loadGateConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sudo: invalid sudo gate config: %v\n", err)
		os.Exit(1)
	}

	resp, err := authorize(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sudo: %v\n%s\n", err, guidance)
		os.Exit(1)
	}
	if !resp.Allow {
		reason := strings.TrimSpace(resp.Reason)
		if reason == "" {
			reason = "sudo is not approved"
		}
		msg := strings.TrimSpace(resp.Guidance)
		if msg == "" {
			msg = guidance
		}
		fmt.Fprintf(os.Stderr, "sudo: %s\n%s\n", reason, msg)
		os.Exit(1)
	}

	argv := append([]string{"sudo"}, os.Args[1:]...)
	if err := syscall.Exec(cfg.RealSudo, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "sudo: failed to execute real sudo: %v\n", err)
		os.Exit(1)
	}
}

func loadGateConfig(path string) (gateConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return gateConfig{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return gateConfig{}, fmt.Errorf("cannot inspect config ownership")
	}
	if stat.Uid != 0 {
		return gateConfig{}, fmt.Errorf("%s must be owned by root", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return gateConfig{}, fmt.Errorf("%s must not be accessible by group or others", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return gateConfig{}, err
	}
	return parseGateConfig(data)
}

func parseGateConfig(data []byte) (gateConfig, error) {
	var cfg gateConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return gateConfig{}, err
	}
	cfg.RealSudo = strings.TrimSpace(cfg.RealSudo)
	cfg.AgentAPIURL = strings.TrimSpace(cfg.AgentAPIURL)
	if err := validateGateConfig(cfg); err != nil {
		return gateConfig{}, err
	}
	return cfg, nil
}

func validateGateConfig(cfg gateConfig) error {
	if cfg.RealSudo == "" {
		return fmt.Errorf("realSudo is required")
	}
	if !filepath.IsAbs(cfg.RealSudo) {
		return fmt.Errorf("realSudo must be an absolute path")
	}
	if cfg.RealSudo == "/usr/bin/sudo" {
		return fmt.Errorf("realSudo must not point at the sudo gate")
	}

	u, err := url.Parse(cfg.AgentAPIURL)
	if err != nil {
		return fmt.Errorf("agentAPIURL is invalid: %w", err)
	}
	if u.Scheme != "http" {
		return fmt.Errorf("agentAPIURL must use http")
	}
	if u.Path != "/sudo/authorize" {
		return fmt.Errorf("agentAPIURL must target /sudo/authorize")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("agentAPIURL must target loopback")
	}
	if u.Port() == "" {
		return fmt.Errorf("agentAPIURL must include a port")
	}
	return nil
}

func authorize(cfg gateConfig) (authorizeResponse, error) {
	cwd, _ := os.Getwd()
	req := authorizeRequest{
		Runtime:      os.Getenv("DISCOBOT_SUDO_RUNTIME"),
		Token:        os.Getenv("DISCOBOT_SUDO_TOKEN"),
		CredentialID: os.Getenv("DISCOBOT_SUDO_CREDENTIAL_ID"),
		UseID:        os.Getenv("DISCOBOT_SUDO_USE_ID"),
		ToolCallID:   os.Getenv("DISCOBOT_SUDO_TOOL_CALL_ID"),
		Command:      os.Getenv("DISCOBOT_SUDO_COMMAND"),
		Cwd:          cwd,
		Args:         os.Args[1:],
		PID:          os.Getpid(),
		PPID:         os.Getppid(),
		TTY:          isTTY(),
		Env:          safeEnv(),
	}
	body, err := json.Marshal(req)
	if err != nil {
		return authorizeResponse{}, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, cfg.AgentAPIURL, bytes.NewReader(body))
	if err != nil {
		return authorizeResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if secret := os.Getenv("DISCOBOT_SECRET"); secret != "" {
		httpReq.Header.Set("Authorization", "Bearer "+secret)
	}

	client := &http.Client{Timeout: defaultTimeout}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return authorizeResponse{}, err
	}
	defer httpResp.Body.Close()

	var resp authorizeResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return authorizeResponse{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return resp, nil
	}
	return resp, nil
}

func isTTY() bool {
	for _, fd := range []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()} {
		var stat syscall.Stat_t
		if err := syscall.Fstat(int(fd), &stat); err == nil && stat.Mode&syscall.S_IFMT == syscall.S_IFCHR {
			return true
		}
	}
	return false
}

func safeEnv() map[string]string {
	allowed := map[string]bool{
		"USER": true, "LOGNAME": true, "HOME": true, "SHELL": true, "TERM": true,
		"PATH": true, "PWD": true, "LANG": true, "LC_ALL": true,
		"DISCOBOT_SUDO_RUNTIME": true, "DISCOBOT_SUDO_CREDENTIAL_ID": true,
		"DISCOBOT_SUDO_USE_ID": true, "DISCOBOT_SUDO_TOOL_CALL_ID": true,
	}
	out := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !allowed[key] {
			continue
		}
		if strings.Contains(strings.ToUpper(key), "TOKEN") || strings.Contains(strings.ToUpper(key), "SECRET") || strings.Contains(strings.ToUpper(key), "PASSWORD") {
			value = "[redacted]"
		}
		out[key] = value
	}
	out["UID"] = strconv.Itoa(os.Getuid())
	out["EUID"] = strconv.Itoa(os.Geteuid())
	if path, err := exec.LookPath("bash"); err == nil {
		out["BASH"] = path
	}
	return out
}
