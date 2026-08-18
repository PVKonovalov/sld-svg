package svgtext

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"unicode"
)

// Report summarizes what InjectFile did to one file, for CLI warnings.
type Report struct {
	Occurrences int
	Applied     int
	Missing     []string // keys with no non-blank translation; left untouched
	Invalid     []string // keys whose translation contained control characters; left untouched
	StaleKeys   []string // keys present in translations but not found in this file
}

// InjectFile re-scans raw with Scan and substitutes, for each occurrence
// whose Key has a non-blank Translation, the translated text in place.
// Occurrences with no (or blank) translation are left byte-identical.
func InjectFile(raw []byte, translations map[string]Entry) ([]byte, Report, error) {
	occs, err := Scan(raw)
	if err != nil {
		return nil, Report{}, err
	}

	var report Report
	report.Occurrences = len(occs)

	used := make(map[string]bool, len(occs))
	missing := make(map[string]bool)
	invalid := make(map[string]bool)

	// occs is already in document order, i.e. sorted by Start with no
	// overlaps, so patches built by iterating it in order satisfy Apply's
	// ordering requirement without an extra sort.
	patches := make([]Patch, 0, len(occs))
	for _, o := range occs {
		used[o.Key] = true
		entry, ok := translations[o.Key]
		if !ok || entry.Translation == "" {
			missing[o.Key] = true
			continue
		}
		if hasControl(entry.Translation) {
			invalid[o.Key] = true
			continue
		}
		var buf bytes.Buffer
		if err := xml.EscapeText(&buf, []byte(entry.Translation)); err != nil {
			return nil, Report{}, fmt.Errorf("svgtext: escaping translation for key %s: %w", o.Key, err)
		}
		patches = append(patches, Patch{Start: o.Start, End: o.End, Replacement: buf.Bytes()})
		report.Applied++
	}

	for k := range missing {
		report.Missing = append(report.Missing, k)
	}
	for k := range invalid {
		report.Invalid = append(report.Invalid, k)
	}
	for k := range translations {
		if !used[k] {
			report.StaleKeys = append(report.StaleKeys, k)
		}
	}

	out, err := Apply(raw, patches)
	if err != nil {
		return nil, Report{}, err
	}
	return out, report, nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
