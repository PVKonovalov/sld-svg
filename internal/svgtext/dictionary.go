package svgtext

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// dictionaryHeader is the expected header of a dictionary file; present for
// readability when opened in a spreadsheet, and skipped on load.
var dictionaryHeader = []string{"source", "translation"}

// DictionaryKey normalizes a string for dictionary lookups, which are
// case- and whitespace-insensitive (but otherwise exact, whole-string
// matches): every whitespace rune is dropped entirely (not just collapsed),
// so e.g. "1СШ 10кВ" and "1 сш 10 кВ" normalize to the same key despite
// differing in exactly where a space falls — a real formatting variance
// seen across this SLD corpus. It is Unicode-aware, so Cyrillic letters
// fold correctly too. Both LoadDictionary and callers looking up a source
// string must use this to normalize keys.
func DictionaryKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// LoadDictionary reads a semicolon-separated "source;translation" lookup
// table (see dictionary.csv) of common strings — units, abbreviations, and
// the like — used by extract to pre-fill the translation column for exact,
// case- and whitespace-insensitive matches, ahead of falling back to
// --ru-en transliteration or leaving it blank. The returned map is keyed by
// DictionaryKey(source).
func LoadDictionary(r io.Reader) (map[string]string, error) {
	cr := csv.NewReader(r)
	cr.Comma = fieldDelim
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("svgtext: reading dictionary: %w", err)
	}

	dict := make(map[string]string, len(records))
	for i, rec := range records {
		if i == 0 && len(rec) == 2 && rec[0] == dictionaryHeader[0] && rec[1] == dictionaryHeader[1] {
			continue
		}
		if len(rec) != 2 {
			return nil, fmt.Errorf("svgtext: dictionary row %d: expected 2 fields, got %d", i+1, len(rec))
		}
		dict[DictionaryKey(rec[0])] = rec[1]
	}
	return dict, nil
}
