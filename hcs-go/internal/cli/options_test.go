package cli

import "testing"

func TestParseMissingValueReturnsError(t *testing.T) {
	if _, err := Parse([]string{"--root"}); err == nil {
		t.Fatal("expected missing value error")
	}
}

func TestParsePlan9ShareAcceptsWindowsPathOnLinux(t *testing.T) {
	options, err := Parse([]string{"--root", `C:\vm\root.vhdx`, "--data", `C:\vm\data.vhdx`, "--share", `src=C:\src`})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Plan9Shares) != 1 || options.Plan9Shares[0].Name != "src" {
		t.Fatalf("unexpected shares: %#v", options.Plan9Shares)
	}
}
