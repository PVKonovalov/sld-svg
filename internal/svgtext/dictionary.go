package svgtext

import (
	"encoding/csv"
	"fmt"
	"io"
)

// dictionaryHeader is the expected header of a dictionary file; present for
// readability when opened in a spreadsheet, and skipped on load.
var dictionaryHeader = []string{"source", "translation"}

// LoadDictionary reads a semicolon-separated "source;translation" lookup
// table (see dictionary.csv) of common strings — units, abbreviations, and
// the like — used by extract to pre-fill the translation column for exact
// matches, ahead of falling back to --ru-en transliteration or leaving it
// blank.
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
		dict[rec[0]] = rec[1]
	}
	return dict, nil
}
