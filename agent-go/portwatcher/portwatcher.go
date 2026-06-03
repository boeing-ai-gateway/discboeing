package portwatcher

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var pidPattern = regexp.MustCompile(`\bpid=([0-9]+)\b`)

const (
	ProtocolHTTP    = "http"
	ProtocolHTTPS   = "https"
	ProtocolUnknown = "unknown"

	probeTimeout = 500 * time.Millisecond
)

// Entry describes one TCP listening socket visible through ss process output.
type Entry struct {
	LocalAddress string `json:"localAddress"`
	Port         int    `json:"port"`
	Process      string `json:"process,omitempty"`
	Protocol     string `json:"protocol"`
	PID          int    `json:"pid"`
	FD           int    `json:"fd,omitempty"`
}

// Scan returns TCP listening sockets with visible process ownership for the current user.
func Scan(ctx context.Context) ([]Entry, error) {
	out, err := exec.CommandContext(ctx, "ss", "-H", "-tnlp").Output()
	if err != nil {
		return nil, fmt.Errorf("run ss -H -tnlp: %w", err)
	}
	entries := ParseSSOutput(string(out))
	DetectProtocols(ctx, entries)
	return entries, nil
}

// ParseSSOutput extracts current-user TCP listeners from ss -H -tnlp output.
func ParseSSOutput(output string) []Entry {
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		entry, ok := parseSSLine(scanner.Text())
		if ok {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Port != entries[j].Port {
			return entries[i].Port < entries[j].Port
		}
		if entries[i].LocalAddress != entries[j].LocalAddress {
			return entries[i].LocalAddress < entries[j].LocalAddress
		}
		return entries[i].PID < entries[j].PID
	})
	return entries
}

func parseSSLine(line string) (Entry, bool) {
	if !pidPattern.MatchString(line) {
		return Entry{}, false
	}

	fields := strings.Fields(line)
	if len(fields) < 4 {
		return Entry{}, false
	}

	localAddress := fields[3]
	port, ok := parsePort(localAddress)
	if !ok {
		return Entry{}, false
	}

	process, pid, fd := parseUsers(line)
	if pid == 0 {
		return Entry{}, false
	}

	return Entry{
		LocalAddress: localAddress,
		Port:         port,
		Process:      process,
		Protocol:     ProtocolUnknown,
		PID:          pid,
		FD:           fd,
	}, true
}

func parsePort(localAddress string) (int, bool) {
	localAddress = strings.TrimSpace(localAddress)
	idx := strings.LastIndex(localAddress, ":")
	if idx < 0 || idx == len(localAddress)-1 {
		return 0, false
	}
	portText := strings.Trim(localAddress[idx+1:], "[]")
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

func parseUsers(line string) (string, int, int) {
	_, after, ok := strings.Cut(line, "users:((\"")
	if !ok {
		return "", 0, 0
	}

	process, _, _ := strings.Cut(after, "\"")

	pid := 0
	if match := pidPattern.FindStringSubmatch(line); len(match) == 2 {
		pid, _ = strconv.Atoi(match[1])
	}

	fd := 0
	if _, after, ok := strings.Cut(line, "fd="); ok {
		fdText := after
		fdEnd := strings.IndexFunc(fdText, func(r rune) bool {
			return r < '0' || r > '9'
		})
		if fdEnd >= 0 {
			fdText = fdText[:fdEnd]
		}
		fd, _ = strconv.Atoi(fdText)
	}

	return process, pid, fd
}

// DetectProtocols probes each entry with short HTTP and HTTPS HEAD requests.
func DetectProtocols(ctx context.Context, entries []Entry) {
	for i := range entries {
		entries[i].Protocol = DetectProtocol(ctx, entries[i])
	}
}

// DetectProtocol returns the first protocol that produces an HTTP response.
func DetectProtocol(ctx context.Context, entry Entry) string {
	target, ok := probeTarget(entry.LocalAddress, entry.Port)
	if !ok {
		return ProtocolUnknown
	}

	if headProbe(ctx, "https://"+target, true) {
		return ProtocolHTTPS
	}
	if headProbe(ctx, "http://"+target, false) {
		return ProtocolHTTP
	}
	return ProtocolUnknown
}

func headProbe(ctx context.Context, url string, insecureTLS bool) bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	transport := &http.Transport{}
	if insecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Timeout:   probeTimeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

func probeTarget(localAddress string, port int) (string, bool) {
	if port <= 0 || port > 65535 {
		return "", false
	}

	host := probeHost(localAddress)
	if host == "" {
		return "", false
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), true
}

func probeHost(localAddress string) string {
	host := strings.TrimSpace(localAddress)
	if host == "" {
		return ""
	}

	if strings.HasPrefix(host, "[") {
		end := strings.LastIndex(host, "]")
		if end < 0 {
			return ""
		}
		host = host[1:end]
	} else if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}

	switch host {
	case "", "*", "0.0.0.0":
		return "127.0.0.1"
	case "::", "[::]":
		return "::1"
	default:
		return host
	}
}
