package cli

import (
	"bytes"
	"testing"
)

func TestPrintTurnInterrupted_TextMidLineAddsLeadingNewline(t *testing.T) {
	md := newMarkdownRenderer(&bytes.Buffer{}, true, false)
	md.WriteText("hello")

	var out bytes.Buffer
	printTurnInterrupted(&out, md, skText)

	if got := out.String(); got != "\n^C\n" {
		t.Fatalf("output = %q, want %q", got, "\n^C\n")
	}
}

func TestPrintTurnInterrupted_LineStartStaysCompact(t *testing.T) {
	md := newMarkdownRenderer(&bytes.Buffer{}, true, false)
	md.WriteText("hello\n")

	var out bytes.Buffer
	printTurnInterrupted(&out, md, skText)

	if got := out.String(); got != "^C\n" {
		t.Fatalf("output = %q, want %q", got, "^C\n")
	}
}

func TestPrintTurnInterrupted_NonTextStaysCompact(t *testing.T) {
	md := newMarkdownRenderer(&bytes.Buffer{}, true, false)

	var out bytes.Buffer
	printTurnInterrupted(&out, md, skTool)

	if got := out.String(); got != "^C\n" {
		t.Fatalf("output = %q, want %q", got, "^C\n")
	}
}
