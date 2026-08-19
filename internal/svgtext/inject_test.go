package svgtext

import (
	"strings"
	"testing"
)

func TestRoundTrip_ExtractTranslateInject(t *testing.T) {
	raw := wrap("", `<rect data-name="Т-1 2,5 МВА"/><text>Label <tspan>кВ</tspan></text><text>Untranslated</text>`)

	entries, err := ExtractFile(raw)
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}

	for k, e := range entries {
		switch e.Source {
		case "Т-1 2,5 МВА":
			e.Translation = "T-1 2.5 MVA"
		case "Label":
			e.Translation = "Label EN"
		case "кВ":
			e.Translation = "kV"
			// "Untranslated" is deliberately left blank.
		}
		entries[k] = e
	}

	out, report, err := InjectFile(raw, entries)
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}

	if report.Applied != 3 {
		t.Fatalf("expected 3 applied, got %d (report=%+v)", report.Applied, report)
	}
	if len(report.Missing) != 1 {
		t.Fatalf("expected 1 missing (Untranslated), got %+v", report.Missing)
	}

	gotStr := string(out)
	for _, want := range []string{"T-1 2.5 MVA", "Label EN", "kV", "Untranslated"} {
		if !strings.Contains(gotStr, want) {
			t.Fatalf("output missing %q: %s", want, gotStr)
		}
	}
	for _, oldText := range []string{"Т-1 2,5 МВА", "Label <", "кВ"} {
		if strings.Contains(gotStr, oldText) {
			t.Fatalf("output still contains untranslated source %q: %s", oldText, gotStr)
		}
	}

	if _, err := Scan(out); err != nil {
		t.Fatalf("Scan(injected output): %v", err)
	}
}

func TestInjectFile_NullTranslationDeletesText(t *testing.T) {
	raw := wrap("", `<text>DD.MM.YYYY</text><rect data-name="DD:MM:YYYY HH:MM:SS"/>`)

	entries, err := ExtractFile(raw)
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	for k, e := range entries {
		switch e.Source {
		case "DD.MM.YYYY":
			e.Translation = "null"
		case "DD:MM:YYYY HH:MM:SS":
			e.Translation = "NULL" // case-insensitive
		}
		entries[k] = e
	}

	out, report, err := InjectFile(raw, entries)
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}
	if report.Applied != 2 {
		t.Fatalf("expected 2 applied, got %d (report=%+v)", report.Applied, report)
	}

	got := string(out)
	for _, want := range []string{"<text></text>", `data-name=""`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %s", want, got)
		}
	}
	for _, oldText := range []string{"DD.MM.YYYY", "DD:MM:YYYY HH:MM:SS"} {
		if strings.Contains(got, oldText) {
			t.Fatalf("output still contains placeholder text %q: %s", oldText, got)
		}
	}
	if _, err := Scan(out); err != nil {
		t.Fatalf("Scan(injected output): %v", err)
	}
}

func TestInjectFile_NoTranslationsLeavesFileUnchanged(t *testing.T) {
	raw := wrap("", `<text>Untouched</text>`)
	out, report, err := InjectFile(raw, map[string]Entry{})
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}
	if string(out) != string(raw) {
		t.Fatalf("expected byte-identical output, got %q want %q", out, raw)
	}
	if report.Applied != 0 || len(report.Missing) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}
