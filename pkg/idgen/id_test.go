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

func TestArgBindsUUIDNotInteger(t *testing.T) {
	id := New()
	arg := Arg(id)
	parsed, ok := arg.(interface{ String() string })
	if !ok || parsed.String() != id {
		t.Fatalf("expected uuid.UUID argument, got %T %v", arg, arg)
	}
	if _, isInt := arg.(int); isInt {
		t.Fatal("uuid query args must not be integers")
	}
	if _, isInt64 := arg.(int64); isInt64 {
		t.Fatal("uuid query args must not be integers")
	}
}
