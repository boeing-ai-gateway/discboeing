package handler

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseExecEventQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/exec/id/events?limit=25&after=7&since=2026-05-06T17:00:00.123456789Z&follow=true", nil)
	query, follow, err := parseExecEventQuery(req)
	if err != nil {
		t.Fatalf("parseExecEventQuery() failed: %v", err)
	}
	if !follow {
		t.Fatal("follow = false, want true")
	}
	if query.Limit != 25 {
		t.Fatalf("limit = %d, want 25", query.Limit)
	}
	if query.After == nil || *query.After != 7 {
		t.Fatalf("after = %v, want 7", query.After)
	}
	wantSince := time.Date(2026, 5, 6, 17, 0, 0, 123456789, time.UTC)
	if query.Since == nil || !query.Since.Equal(wantSince) {
		t.Fatalf("since = %v, want %v", query.Since, wantSince)
	}
}

func TestParseExecEventQueryUsesLastEventID(t *testing.T) {
	req := httptest.NewRequest("GET", "/exec/id/events?follow=true", nil)
	req.Header.Set("Last-Event-ID", "42")
	query, _, err := parseExecEventQuery(req)
	if err != nil {
		t.Fatalf("parseExecEventQuery() failed: %v", err)
	}
	if query.After == nil || *query.After != 42 {
		t.Fatalf("after = %v, want 42", query.After)
	}
}
