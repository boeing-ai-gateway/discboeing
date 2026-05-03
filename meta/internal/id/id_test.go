package id

import (
	"strings"
	"testing"
)

func TestNewUsesRegisteredPrefix(t *testing.T) {
	got, err := New(TypeSecret)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "sec_") {
		t.Fatalf("id %q does not have secret prefix", got)
	}
	suffix := strings.TrimPrefix(got, "sec_")
	if len(suffix) != 26 {
		t.Fatalf("id %q does not have 26 character ULID suffix", got)
	}
	if suffix != strings.ToLower(suffix) {
		t.Fatalf("id %q does not have lowercase ULID suffix", got)
	}
}

func TestNewRejectsUnknownType(t *testing.T) {
	_, err := New(Type("missing"))
	if err == nil {
		t.Fatal("expected unknown type error")
	}
}

func TestRandomCrockford(t *testing.T) {
	got, err := RandomCrockford(52)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 52 {
		t.Fatalf("length = %d, want 52", len(got))
	}
	for _, r := range got {
		if !strings.ContainsRune(CrockfordBase32, r) {
			t.Fatalf("unexpected rune %q in %q", r, got)
		}
	}
	if strings.ContainsAny(got, "-_") {
		t.Fatalf("random crockford token contains separator char: %q", got)
	}
}

func TestPrefixRegistry(t *testing.T) {
	prefix, ok := Prefix(TypeAgentSession)
	if !ok {
		t.Fatal("expected agent session prefix")
	}
	if prefix != "ags" {
		t.Fatalf("prefix = %q, want ags", prefix)
	}

	prefix, ok = Prefix(TypeOrganization)
	if !ok {
		t.Fatal("expected organization prefix")
	}
	if prefix != "org" {
		t.Fatalf("prefix = %q, want org", prefix)
	}
}
