package svgtext

import (
	"strings"
	"testing"
)

func TestLoadDictionary(t *testing.T) {
	src := "source;translation\nГц;Hz\nВ;V\n"
	dict, err := LoadDictionary(strings.NewReader(src))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	want := map[string]string{"Гц": "Hz", "В": "V"}
	if len(dict) != len(want) {
		t.Fatalf("got %+v want %+v", dict, want)
	}
	for k, v := range want {
		if dict[DictionaryKey(k)] != v {
			t.Fatalf("dict[%q] = %q, want %q", k, dict[DictionaryKey(k)], v)
		}
	}
}

func TestLoadDictionary_NoHeader(t *testing.T) {
	dict, err := LoadDictionary(strings.NewReader("Гц;Hz\n"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	if dict[DictionaryKey("Гц")] != "Hz" {
		t.Fatalf("got %+v", dict)
	}
}

func TestLoadDictionary_CaseInsensitiveLookup(t *testing.T) {
	dict, err := LoadDictionary(strings.NewReader("source;translation\n1СШ 10кВ;1SEC 10kV\n"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	for _, variant := range []string{"1СШ 10кВ", "1сш 10кв", "1Сш 10Кв"} {
		if got := dict[DictionaryKey(variant)]; got != "1SEC 10kV" {
			t.Fatalf("dict[DictionaryKey(%q)] = %q, want %q", variant, got, "1SEC 10kV")
		}
	}
}

func TestLoadDictionary_WhitespaceInsensitiveLookup(t *testing.T) {
	dict, err := LoadDictionary(strings.NewReader("source;translation\n1СШ 10кВ;1SEC 10kV\n"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	// Same tokens, different placement/amount of whitespace: this is a real
	// formatting variance seen across the SLD corpus, not a hypothetical.
	for _, variant := range []string{"1 сш 10 кВ", "1сш10кв", "1  СШ 10 КВ", "1СШ10кВ"} {
		if got := dict[DictionaryKey(variant)]; got != "1SEC 10kV" {
			t.Fatalf("dict[DictionaryKey(%q)] = %q, want %q", variant, got, "1SEC 10kV")
		}
	}
}
