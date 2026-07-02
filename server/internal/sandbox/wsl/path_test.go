package wsl

import "testing"

func TestTranslatePath(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		want    string
		wantErr bool
	}{
		{
			name:   "unix path unchanged",
			source: "/home/discboeing/workspace",
			want:   "/home/discboeing/workspace",
		},
		{
			name:   "windows drive path",
			source: `C:\Users\me\repo`,
			want:   "/mnt/c/Users/me/repo",
		},
		{
			name:   "windows drive path with forward slashes",
			source: `D:/code/proj`,
			want:   "/mnt/d/code/proj",
		},
		{
			name:   "windows root path",
			source: `E:\`,
			want:   "/mnt/e",
		},
		{
			name:    "relative path rejected",
			source:  `repo`,
			wantErr: true,
		},
		{
			name:    "drive relative path rejected",
			source:  `C:repo`,
			wantErr: true,
		},
		{
			name:    "unc path rejected",
			source:  `\\server\share\repo`,
			wantErr: true,
		},
		{
			name:    "device path rejected",
			source:  `\\?\C:\repo`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TranslatePath(tt.source)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TranslatePath(%q) error = nil, want error", tt.source)
				}
				return
			}
			if err != nil {
				t.Fatalf("TranslatePath(%q) error = %v", tt.source, err)
			}
			if got != tt.want {
				t.Fatalf("TranslatePath(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}
