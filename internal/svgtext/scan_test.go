package svgtext

import "testing"

func wrap(rootAttrs, body string) []byte {
	return []byte(`<?xml version="1.0"?>` + "\n" +
		`<svg xmlns="http://www.w3.org/2000/svg"` + rootAttrs + `>` + body + `</svg>`)
}

func mustScan(t *testing.T, raw []byte) []Occurrence {
	t.Helper()
	occs, err := Scan(raw)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return occs
}

func sources(occs []Occurrence) []string {
	out := make([]string, len(occs))
	for i, o := range occs {
		out[i] = o.Source
	}
	return out
}

func TestScan_DataNameOnNonTextElement(t *testing.T) {
	raw := wrap("", `<rect x="1" y="2" data-name="Т-1 2,5 МВА"/>`)
	occs := mustScan(t, raw)
	if len(occs) != 1 || occs[0].Kind != KindDataName || occs[0].Source != "Т-1 2,5 МВА" {
		t.Fatalf("got %+v", occs)
	}
	if got := string(raw[occs[0].Start:occs[0].End]); got != occs[0].Source {
		t.Fatalf("byte span mismatch: got %q want %q", got, occs[0].Source)
	}
}

func TestScan_TextNoTspan(t *testing.T) {
	raw := wrap("", `<text>Р Т-1 10</text>`)
	occs := mustScan(t, raw)
	if len(occs) != 1 || occs[0].Kind != KindText || occs[0].Source != "Р Т-1 10" {
		t.Fatalf("got %+v", occs)
	}
}

func TestScan_TextWithOneTspan_FiltersNumericOuter(t *testing.T) {
	raw := wrap("", `<text>0 <tspan>кВ</tspan></text>`)
	occs := mustScan(t, raw)
	if len(occs) != 1 || occs[0].Kind != KindText || occs[0].Source != "кВ" {
		t.Fatalf("got %+v", occs)
	}
}

func TestScan_TextWithThreeTspans(t *testing.T) {
	raw := wrap("", `<text>Label <tspan>A</tspan> <tspan>B</tspan> <tspan>C</tspan></text>`)
	occs := mustScan(t, raw)
	got := sources(occs)
	want := []string{"Label", "A", "B", "C"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestScan_EntityEncodedQuote(t *testing.T) {
	raw := wrap("", `<text>ООО &#34;Пример&#34;</text>`)
	occs := mustScan(t, raw)
	want := `ООО "Пример"`
	if len(occs) != 1 || occs[0].Source != want {
		t.Fatalf("got %+v want source %q", occs, want)
	}
}

func TestScan_NumericOnlyTextIsFiltered(t *testing.T) {
	raw := wrap("", `<text>0.00 </text>`)
	occs := mustScan(t, raw)
	if len(occs) != 0 {
		t.Fatalf("expected zero occurrences, got %+v", occs)
	}
}

func TestScan_MetadataBlockIgnored(t *testing.T) {
	raw := wrap("", `<metadata>{"layers":[{"id":"1","label":"Мощность"}]}</metadata>`)
	occs := mustScan(t, raw)
	if len(occs) != 0 {
		t.Fatalf("expected zero occurrences, got %+v", occs)
	}
}

func TestScan_CommentWithLiteralDataNameIgnored(t *testing.T) {
	raw := wrap("", `<!-- data-name="x" --><rect x="1"/>`)
	occs := mustScan(t, raw)
	if len(occs) != 0 {
		t.Fatalf("expected zero occurrences, got %+v", occs)
	}
}

func TestScan_SingleQuotedRootAttributeDoesNotMisfire(t *testing.T) {
	raw := wrap(` style='fill:red'`, `<text>Hello Мир</text>`)
	occs := mustScan(t, raw)
	if len(occs) != 1 || occs[0].Source != "Hello Мир" {
		t.Fatalf("got %+v", occs)
	}
}

func TestScan_AdversarialAttributeSubstring(t *testing.T) {
	raw := wrap("", `<rect foo-data-name="zzz" data-name="real"/>`)
	occs := mustScan(t, raw)
	if len(occs) != 1 || occs[0].Source != "real" {
		t.Fatalf("got %+v", occs)
	}
}

func TestExtractFile_DedupesWithinFile(t *testing.T) {
	raw := wrap("", `<rect data-name="Same Label"/><text>Same Label</text>`)
	entries, err := ExtractFile(raw)
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 deduped entry, got %d: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if e.Source != "Same Label" {
			t.Fatalf("got %+v", e)
		}
	}
}
