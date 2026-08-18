package svgtext

import "fmt"

// Patch replaces raw[Start:End] with Replacement.
type Patch struct {
	Start, End  int
	Replacement []byte
}

// Apply rebuilds file contents from raw, copying bytes outside each patch
// verbatim and splicing in each patch's Replacement. Patches must be sorted
// by Start and non-overlapping; Apply returns an error rather than silently
// mis-splicing if that invariant is violated. Zero patches yields a
// byte-identical copy of raw.
func Apply(raw []byte, patches []Patch) ([]byte, error) {
	out := make([]byte, 0, len(raw))
	pos := 0
	for i, p := range patches {
		if p.Start < pos || p.End < p.Start || p.End > len(raw) {
			return nil, fmt.Errorf("svgtext: patch %d [%d,%d) is out of order or overlapping (previous end %d, file length %d)", i, p.Start, p.End, pos, len(raw))
		}
		out = append(out, raw[pos:p.Start]...)
		out = append(out, p.Replacement...)
		pos = p.End
	}
	out = append(out, raw[pos:]...)
	return out, nil
}
