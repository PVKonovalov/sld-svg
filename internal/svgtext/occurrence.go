// Package svgtext extracts and re-injects translatable text embedded in
// single-line-diagram SVG files, at the byte level, preserving everything
// else in the file untouched.
package svgtext

// Kind identifies where an Occurrence's text came from.
type Kind uint8

const (
	// KindDataName is a data-name="..." attribute value, found on any element.
	KindDataName Kind = iota
	// KindText is CharData whose parent element is <text> or <tspan>.
	KindText
)

// Occurrence is one translatable location found in an SVG file.
type Occurrence struct {
	Kind Kind
	// Key is HashKey(Source).
	Key string
	// Source is the trimmed, entity-decoded original text.
	Source string
	// Start and End are the byte offsets in the original file's bytes of
	// the exact span to replace on injection. [Start, End) never includes
	// surrounding whitespace or quoting.
	Start int
	End   int
}
