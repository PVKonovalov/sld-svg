package svgtext

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"unicode"
)

// dataNameAttrRe locates a data-name="..." attribute's value within a raw
// start-tag byte slice. The (^|\s) prefix requires the attribute name to
// start at a word boundary, so it can't match inside a longer attribute
// name like foo-data-name. A literal '"' can never appear unescaped inside
// a double-quoted XML attribute value, so [^"]* can't overrun into the
// wrong attribute.
var dataNameAttrRe = regexp.MustCompile(`(^|\s)data-name="([^"]*)"`)

// Scan performs a single streaming pass over an SVG file's raw bytes and
// returns every translatable occurrence, in document order, as
// non-overlapping byte spans into raw. It filters out any candidate string
// with no letter rune (numeric/placeholder values like "0.00 ").
//
// extract and inject both call Scan on the same bytes and rely on it being
// fully deterministic: inject re-derives the same Key from whatever is
// currently in the file and looks up its translation by that key alone.
func Scan(raw []byte) ([]Occurrence, error) {
	dec := xml.NewDecoder(bytes.NewReader(raw))

	var occs []Occurrence
	var stack []string

	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("svgtext: scan at byte %d: %w", start, err)
		}
		end := dec.InputOffset()

		switch t := tok.(type) {
		case xml.StartElement:
			occ, ok, err := dataNameOccurrence(raw, int(start), int(end), t)
			if err != nil {
				return nil, err
			}
			if ok {
				occs = append(occs, occ)
			}
			stack = append(stack, t.Name.Local)

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if parent == "text" || parent == "tspan" {
					occ, ok, err := textOccurrence(raw, int(start), int(end))
					if err != nil {
						return nil, err
					}
					if ok {
						occs = append(occs, occ)
					}
				}
			}
		}
	}

	return occs, nil
}

func dataNameOccurrence(raw []byte, start, end int, t xml.StartElement) (Occurrence, bool, error) {
	var attrVal string
	found := false
	for _, a := range t.Attr {
		if a.Name.Local == "data-name" {
			attrVal = a.Value
			found = true
			break
		}
	}
	if !found {
		return Occurrence{}, false, nil
	}

	tagBytes := raw[start:end]
	loc := dataNameAttrRe.FindSubmatchIndex(tagBytes)
	if loc == nil {
		return Occurrence{}, false, fmt.Errorf("svgtext: could not locate data-name attribute value in tag at byte offset %d", start)
	}
	valStart, valEnd := loc[4], loc[5]
	rawVal := tagBytes[valStart:valEnd]

	fullDecoded, err := decodeAttrValue(rawVal)
	if err != nil {
		return Occurrence{}, false, fmt.Errorf("svgtext: decoding data-name value at byte offset %d: %w", start+valStart, err)
	}
	if fullDecoded != attrVal {
		return Occurrence{}, false, fmt.Errorf("svgtext: data-name value mismatch at byte offset %d: decoder=%q regex-located=%q", start, attrVal, fullDecoded)
	}

	coreStart, coreEnd := trimWhitespaceSpan(rawVal)
	if coreStart == coreEnd {
		return Occurrence{}, false, nil
	}
	decoded, err := decodeAttrValue(rawVal[coreStart:coreEnd])
	if err != nil {
		return Occurrence{}, false, fmt.Errorf("svgtext: decoding data-name value at byte offset %d: %w", start+valStart+coreStart, err)
	}
	if !hasLetter(decoded) {
		return Occurrence{}, false, nil
	}

	return Occurrence{
		Kind:   KindDataName,
		Key:    HashKey(decoded),
		Source: decoded,
		Start:  start + valStart + coreStart,
		End:    start + valStart + coreEnd,
	}, true, nil
}

func textOccurrence(raw []byte, start, end int) (Occurrence, bool, error) {
	tokenBytes := raw[start:end]
	coreStart, coreEnd := trimWhitespaceSpan(tokenBytes)
	if coreStart == coreEnd {
		return Occurrence{}, false, nil
	}
	decoded, err := decodeTextFragment(tokenBytes[coreStart:coreEnd])
	if err != nil {
		return Occurrence{}, false, fmt.Errorf("svgtext: decoding text at byte offset %d: %w", start+coreStart, err)
	}
	if !hasLetter(decoded) {
		return Occurrence{}, false, nil
	}
	return Occurrence{
		Kind:   KindText,
		Key:    HashKey(decoded),
		Source: decoded,
		Start:  start + coreStart,
		End:    start + coreEnd,
	}, true, nil
}

// decodeAttrValue decodes a raw (still-escaped) XML attribute value
// fragment using encoding/xml itself, so entity handling stays byte-for-byte
// identical to the main parse.
func decodeAttrValue(raw []byte) (string, error) {
	doc := make([]byte, 0, len(raw)+8)
	doc = append(doc, `<a v="`...)
	doc = append(doc, raw...)
	doc = append(doc, `"/>`...)
	var x struct {
		V string `xml:"v,attr"`
	}
	if err := xml.Unmarshal(doc, &x); err != nil {
		return "", err
	}
	return x.V, nil
}

// decodeTextFragment decodes a raw (still-escaped) XML character-data
// fragment the same way.
func decodeTextFragment(raw []byte) (string, error) {
	doc := make([]byte, 0, len(raw)+7)
	doc = append(doc, `<a>`...)
	doc = append(doc, raw...)
	doc = append(doc, `</a>`...)
	var x struct {
		V string `xml:",chardata"`
	}
	if err := xml.Unmarshal(doc, &x); err != nil {
		return "", err
	}
	return x.V, nil
}

// trimWhitespaceSpan returns the [start, end) sub-range of b with leading
// and trailing ASCII whitespace bytes removed. It trims on raw bytes, not a
// decoded string, so it never trims into the middle of an entity reference:
// entity references never contain whitespace bytes internally, and an
// entity-encoded space (e.g. "&#32;") is not itself a whitespace byte, so
// it is correctly left as part of the core span.
func trimWhitespaceSpan(b []byte) (start, end int) {
	start = 0
	for start < len(b) && isASCIISpace(b[start]) {
		start++
	}
	end = len(b)
	for end > start && isASCIISpace(b[end-1]) {
		end--
	}
	return start, end
}

func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
