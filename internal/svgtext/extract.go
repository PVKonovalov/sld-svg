package svgtext

// ExtractFile scans one SVG file's bytes and returns its translatable
// strings keyed by HashKey, with Translation left blank. Duplicate
// occurrences of the same source string within the file collapse to a
// single Entry.
func ExtractFile(raw []byte) (map[string]Entry, error) {
	occs, err := Scan(raw)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]Entry, len(occs))
	for _, o := range occs {
		if _, ok := entries[o.Key]; ok {
			continue
		}
		entries[o.Key] = Entry{Key: o.Key, Source: o.Source}
	}
	return entries, nil
}
