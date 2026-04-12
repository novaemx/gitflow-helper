package branch

import "testing"

func TestTypeOf(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"feature/foo", "feature"},
		{"bugfix/bar", "bugfix"},
		{"release/1.0.0", "release"},
		{"hotfix/1.0.1", "hotfix"},
		{"develop", "base"},
		{"main", "base"},
		{"master", "base"},
		{"random", "other"},
		{"", "other"},
		{"feature", "other"},
	}
	for _, tc := range cases {
		got := TypeOf(tc.name)
		if got != tc.want {
			t.Errorf("TypeOf(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
