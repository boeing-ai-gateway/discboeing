package services

import (
	"strings"
	"testing"
)

func TestConnectionRefusedHTMLReloadsOnceProxyStopsReturningStartupErrors(t *testing.T) {
	html := connectionRefusedHTML(3015)

	if !strings.Contains(html, "res.status !== 502 && res.status !== 503") {
		t.Fatalf("expected startup page to reload once the proxy stops returning 502/503, got:\n%s", html)
	}

	if !strings.Contains(html, "cache: 'no-store'") {
		t.Fatalf("expected startup page retry fetch to bypass caches, got:\n%s", html)
	}
}
