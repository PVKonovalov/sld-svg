package svgtext

import "testing"

func TestHashKey_Deterministic(t *testing.T) {
	a := HashKey("Т-1 2,5 МВА")
	b := HashKey("Т-1 2,5 МВА")
	if a != b {
		t.Fatalf("HashKey not deterministic: %q vs %q", a, b)
	}
	if HashKey("different") == a {
		t.Fatalf("HashKey collided unexpectedly")
	}
	if len(a) != 16 {
		t.Fatalf("expected 16 hex chars, got %d (%q)", len(a), a)
	}
}
