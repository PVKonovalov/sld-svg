package svgtext

import (
	"bytes"
	"testing"
)

func TestTranslationsRoundTrip(t *testing.T) {
	entries := map[string]Entry{
		"k1": {Key: "k1", Source: "Hello", Translation: "Bonjour"},
		"k2": {Key: "k2", Source: "Semicolon; and, comma", Translation: ""},
		"k3": {Key: "k3", Source: "Multi\nline\ttext", Translation: "Quoted; safely"},
	}
	var buf bytes.Buffer
	if err := SaveTranslations(&buf, entries); err != nil {
		t.Fatalf("SaveTranslations: %v", err)
	}
	got, err := LoadTranslations(&buf)
	if err != nil {
		t.Fatalf("LoadTranslations: %v", err)
	}
	if len(got) != len(entries) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(entries), got)
	}
	for k, want := range entries {
		if got[k] != want {
			t.Fatalf("entry %s: got %+v want %+v", k, got[k], want)
		}
	}
}

func TestLoadTranslations_EmptyFile(t *testing.T) {
	got, err := LoadTranslations(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("LoadTranslations: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveTranslations_UsesSemicolonDelimiter(t *testing.T) {
	var buf bytes.Buffer
	entries := map[string]Entry{
		"k1": {Key: "k1", Source: "Т-1 2,5 МВА", Translation: "T-1 2.5 MVA"},
	}
	if err := SaveTranslations(&buf, entries); err != nil {
		t.Fatalf("SaveTranslations: %v", err)
	}
	got := buf.String()
	want := "key;source;translation\nk1;Т-1 2,5 МВА;T-1 2.5 MVA\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
