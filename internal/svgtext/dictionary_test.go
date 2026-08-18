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
		if dict[k] != v {
			t.Fatalf("dict[%q] = %q, want %q", k, dict[k], v)
		}
	}
}

func TestLoadDictionary_NoHeader(t *testing.T) {
	dict, err := LoadDictionary(strings.NewReader("Гц;Hz\n"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}
	if dict["Гц"] != "Hz" {
		t.Fatalf("got %+v", dict)
	}
}
