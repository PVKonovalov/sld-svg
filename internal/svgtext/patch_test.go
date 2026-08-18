package svgtext

import "testing"

func TestApply_ReplacesAndPreservesSurroundingBytes(t *testing.T) {
	raw := []byte("abcXYZdef")
	out, err := Apply(raw, []Patch{{Start: 3, End: 6, Replacement: []byte("123")}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if string(out) != "abc123def" {
		t.Fatalf("got %q", out)
	}
}

func TestApply_NoPatchesIsIdentity(t *testing.T) {
	raw := []byte("hello world")
	out, err := Apply(raw, nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if string(out) != string(raw) {
		t.Fatalf("got %q want %q", out, raw)
	}
}

func TestApply_OverlappingPatchesError(t *testing.T) {
	raw := []byte("abcdef")
	_, err := Apply(raw, []Patch{
		{Start: 0, End: 3, Replacement: []byte("X")},
		{Start: 2, End: 4, Replacement: []byte("Y")},
	})
	if err == nil {
		t.Fatalf("expected error for overlapping patches")
	}
}
