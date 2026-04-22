package main

import "testing"

func TestNormalizeCommitHash(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "none", in: "none", want: ""},
		{name: "unknown", in: "unknown", want: ""},
		{name: "invalid", in: "not-a-hash", want: ""},
		{name: "short valid", in: "a1b2c3d", want: "a1b2c3d"},
		{name: "full hash trimmed", in: "0123456789abcdef0123456789abcdef01234567", want: "0123456789ab"},
		{name: "mixed case normalized", in: "ABCDEF123456", want: "abcdef123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCommitHash(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeCommitHash(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildDisplayVersion(t *testing.T) {
	if got := buildDisplayVersion("0.5.40", ""); got != "0.5.40" {
		t.Fatalf("expected plain version, got %q", got)
	}

	if got := buildDisplayVersion("0.5.40", "abcdef123456"); got != "0.5.40 (build abcdef123456)" {
		t.Fatalf("expected build hash version, got %q", got)
	}
}
