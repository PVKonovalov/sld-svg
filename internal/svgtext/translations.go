package svgtext

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
)

// Entry is one row of a translation file: a source string keyed by
// HashKey(Source), and its (possibly still-empty) Translation.
type Entry struct {
	Key         string
	Source      string
	Translation string
}

// NullTranslation is a sentinel Translation value meaning "replace this
// occurrence with an empty string" — as opposed to a blank Translation,
// which means "not yet translated, leave the original text untouched."
// Matched case-insensitively. Useful for placeholder/template text (e.g. a
// live clock's "DD.MM.YYYY" label) that should simply be removed rather
// than translated.
const NullTranslation = "null"

// fieldDelim is ';' rather than a tab: source strings in this SLD corpus
// commonly contain literal commas (Russian decimal notation, e.g. "2,5"),
// but never semicolons, so this needs almost no CSV quoting in practice —
// and unlike a tab, a semicolon can't be silently mangled into spaces by an
// editor's "insert spaces for tabs" setting.
const fieldDelim = ';'

var csvHeader = []string{"key", "source", "translation"}

// LoadTranslations reads a translation file previously written by
// SaveTranslations, keyed by Entry.Key.
func LoadTranslations(r io.Reader) (map[string]Entry, error) {
	cr := csv.NewReader(r)
	cr.Comma = fieldDelim
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("svgtext: reading translation file: %w", err)
	}

	entries := make(map[string]Entry, len(records))
	for i, rec := range records {
		if i == 0 && rec[0] == csvHeader[0] && rec[1] == csvHeader[1] && rec[2] == csvHeader[2] {
			continue
		}
		entries[rec[0]] = Entry{Key: rec[0], Source: rec[1], Translation: rec[2]}
	}
	return entries, nil
}

// SaveTranslations writes entries as a semicolon-separated file, sorted by
// Source (then Key, for determinism) so it reads naturally for a human
// translator and can be freely reordered by hand without affecting
// correctness.
func SaveTranslations(w io.Writer, entries map[string]Entry) error {
	list := make([]Entry, 0, len(entries))
	for _, e := range entries {
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Source != list[j].Source {
			return list[i].Source < list[j].Source
		}
		return list[i].Key < list[j].Key
	})

	cw := csv.NewWriter(w)
	cw.Comma = fieldDelim
	if err := cw.Write(csvHeader); err != nil {
		return fmt.Errorf("svgtext: writing translation file: %w", err)
	}
	for _, e := range list {
		if err := cw.Write([]string{e.Key, e.Source, e.Translation}); err != nil {
			return fmt.Errorf("svgtext: writing translation file: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}
