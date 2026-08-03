package main

import (
	"path/filepath"
	"testing"
)

// TestResolveStorePath exercises the boundary check idAndSort relies on
// (issue #202) to stop a filter plugin's attacker-controlled entry.Path from
// writing outside fileSystemBaseDir.
func TestResolveStorePath(t *testing.T) {
	base := filepath.FromSlash("/app/store")

	tests := []struct {
		name    string
		rel     string
		want    string
		wantErr bool
	}{
		{
			name: "plain relative path stays under base",
			rel:  "2024/claim.pdf",
			want: filepath.FromSlash("/app/store/2024/claim.pdf"),
		},
		{
			name: "empty path resolves to base itself",
			rel:  "",
			want: base,
		},
		{
			name:    "traversal escaping base entirely",
			rel:     "../../etc/cron.d/x",
			wantErr: true,
		},
		{
			name:    "traversal landing on a sibling directory sharing base's name as a prefix",
			rel:     "../store-evil/x",
			wantErr: true,
		},
		{
			name: "traversal that stays inside base after cleaning",
			rel:  "2024/../2025/claim.pdf",
			want: filepath.FromSlash("/app/store/2025/claim.pdf"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveStorePath(base, tt.rel)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveStorePath(%q, %q) = %q, want error", base, tt.rel, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveStorePath(%q, %q) unexpected error: %v", base, tt.rel, err)
			}
			if got != tt.want {
				t.Fatalf("resolveStorePath(%q, %q) = %q, want %q", base, tt.rel, got, tt.want)
			}
		})
	}
}
