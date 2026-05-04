package ide

import "testing"

func TestWithVersionHeaderAddsFirstLine(t *testing.T) {
	SetGeneratorVersion("1.2.3")
	got := withVersionHeader("# Header\n\nBody\n")
	if firstLine(got) != "<!-- gitflow-version: 1.2.3 -->" {
		t.Fatalf("unexpected first line: %q", firstLine(got))
	}
}

func TestWithVersionHeaderReplacesExistingVersionLine(t *testing.T) {
	SetGeneratorVersion("2.0.0")
	content := "<!-- gitflow-version: 1.0.0 -->\n# Header\n"
	got := withVersionHeader(content)
	if firstLine(got) != "<!-- gitflow-version: 2.0.0 -->" {
		t.Fatalf("expected updated version line, got %q", firstLine(got))
	}
}

func TestParseVersionFromLine(t *testing.T) {
	v, ok := parseVersionFromLine("<!-- gitflow-version: v0.6.2 -->")
	if !ok || v != "v0.6.2" {
		t.Fatalf("expected v0.6.2 parsed, got ok=%v v=%q", ok, v)
	}
}

func TestIsOlderVersion(t *testing.T) {
	if !isOlderVersion("0.6.1", "0.6.2") {
		t.Fatal("expected older version to require refresh")
	}
	if isOlderVersion("0.6.2", "0.6.1") {
		t.Fatal("did not expect newer stored version to refresh")
	}
	if isOlderVersion("0.6.2", "0.6.2") {
		t.Fatal("did not expect equal versions to refresh")
	}
}
