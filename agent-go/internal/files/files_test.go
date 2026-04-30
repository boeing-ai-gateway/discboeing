package files

import "testing"

func TestNormalizeResultPath(t *testing.T) {
	t.Run("empty becomes dot", func(t *testing.T) {
		if got := normalizeResultPath(""); got != "." {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("dot stays dot", func(t *testing.T) {
		if got := normalizeResultPath("."); got != "." {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("windows separators become slashes", func(t *testing.T) {
		if got := normalizeResultPath(`artifacts\browser\sha256\shot.png`); got != "artifacts/browser/sha256/shot.png" {
			t.Fatalf("got %q", got)
		}
	})
}
