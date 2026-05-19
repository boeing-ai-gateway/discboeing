package classnames

import (
	"strings"
	"testing"
)

func TestCNMergesTailwindConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "responsive max width override",
			in: []string{
				"fixed w-full sm:max-w-lg",
				"sm:max-w-2xl max-h-[90vh]",
			},
			want: []string{"fixed", "w-full", "sm:max-w-2xl", "max-h-[90vh]"},
		},
		{
			name: "padding conflicts preserve partial override",
			in:   []string{"p-2", "px-4"},
			want: []string{"p-2", "px-4"},
		},
		{
			name: "later padding shorthand wins",
			in:   []string{"px-4", "p-2"},
			want: []string{"p-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertClassSet(t, CN(tt.in...), tt.want)
		})
	}
}

func assertClassSet(t *testing.T, got string, want []string) {
	t.Helper()

	gotClasses := strings.Fields(got)
	if len(gotClasses) != len(want) {
		t.Fatalf("CN() = %q, want classes %v", got, want)
	}

	seen := make(map[string]bool, len(gotClasses))
	for _, class := range gotClasses {
		seen[class] = true
	}
	for _, class := range want {
		if !seen[class] {
			t.Fatalf("CN() = %q, missing class %q", got, class)
		}
	}
}
