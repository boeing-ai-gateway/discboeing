package state

import "testing"

func TestDeriveFileGitStatusFromPath(t *testing.T) {
	status := DeriveFileGitStatusFromPath("content/file_tree.templ", map[string]FileGitStatus{
		"content/file_tree.templ": FileGitStatusModified,
	})
	if status != FileGitStatusModified {
		t.Fatalf("status = %q, want modified", status)
	}
	if clean := DeriveFileGitStatusFromPath("missing", nil); clean != FileGitStatusClean {
		t.Fatalf("missing status = %q, want clean", clean)
	}
}
