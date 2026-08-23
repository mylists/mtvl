package idgen

import "testing"

func TestNewIsUniqueUUID(t *testing.T) {
	a := New()
	b := New()
	if a == b {
		t.Fatalf("expected unique ids, got %q twice", a)
	}
	if parsed, ok := Parse(a); !ok || parsed != a {
		t.Fatalf("generated id should parse: %q", a)
	}
}

func TestParseRejectsIncrementalIDs(t *testing.T) {
	if _, ok := Parse("12"); ok {
		t.Fatal("expected incremental id to be rejected")
	}
	if _, ok := Parse(""); ok {
		t.Fatal("expected empty id to be rejected")
	}
}
