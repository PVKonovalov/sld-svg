package slddoc

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

const testSymbols = `<symbols>
  <symbol shape="41">
    <template><![CDATA[
<path d="M -7 -7 h 14 v 14 h -14 z" style="fill:{fill};stroke:{color};stroke-width:1" />
]]></template>
  </symbol>
</symbols>`

func TestLoadSymbolLibrary(t *testing.T) {
	lib, err := LoadSymbolLibrary(strings.NewReader(testSymbols))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lib.templates["41"]; !ok {
		t.Fatalf("templates = %+v, want shape 41", lib.templates)
	}
}

func TestRender_ProducesWellFormedSVG(t *testing.T) {
	lib, err := LoadSymbolLibrary(strings.NewReader(testSymbols))
	if err != nil {
		t.Fatal(err)
	}

	state := 1
	d := &Diagram{
		Width: 100, Height: 100,
		VoltageClasses: []VoltageClass{{ID: "v1", Name: "10кВ", Color: "#962896"}},
		Elements: []Element{
			{ID: "bus", Class: ClassBusBarSection, Voltage: "v1", Points: []Point{{0, 0}, {100, 0}}},
			{ID: "brk", Class: ClassBreaker, Shape: "41", Name: "В-1", Voltage: "v1", X: 50, Y: 50, State: &state},
		},
		Connectors: []Connector{
			{ID: "w1", Voltage: "v1", Points: []Point{{50, 0}, {50, 43}}},
		},
		Labels: []Label{{X: 10, Y: 10, Size: 13, Text: "line one\nline two"}},
	}

	var buf bytes.Buffer
	if err := Render(d, lib, &buf); err != nil {
		t.Fatal(err)
	}

	var probe struct {
		XMLName xml.Name `xml:"svg"`
	}
	if err := xml.Unmarshal(buf.Bytes(), &probe); err != nil {
		t.Fatalf("rendered output is not well-formed XML: %v\n%s", err, buf.String())
	}

	out := buf.String()
	if !strings.Contains(out, `stroke:#962896`) {
		t.Errorf("voltage color not applied: %s", out)
	}
	if !strings.Contains(out, `fill:lawngreen`) {
		t.Errorf("state 1 should render lawngreen fill: %s", out)
	}
	if !strings.Contains(out, `translate(50,50) rotate(0)`) {
		t.Errorf("element not placed at its anchor: %s", out)
	}
	if !strings.Contains(out, "<tspan") {
		t.Errorf("multi-line label should emit a tspan: %s", out)
	}
}

func TestApplyStateLine(t *testing.T) {
	tmpl := `<path d="{state:PARALLEL|PERPENDICULAR|DIAGONAL}" />`
	one, zero, two, none := 1, 0, 2, (*int)(nil)

	cases := []struct {
		name  string
		state *int
		want  string
	}{
		{"closed", &one, "PARALLEL"},
		{"open", &zero, "PERPENDICULAR"},
		{"undefined", &two, "DIAGONAL"},
		{"unrecorded defaults to parallel", none, "PARALLEL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := applyStateLine(tmpl, c.state)
			if !strings.Contains(got, c.want) {
				t.Errorf("applyStateLine(%v) = %q, want it to contain %q", c.state, got, c.want)
			}
		})
	}
}

func TestRender_ReportsMissingShape(t *testing.T) {
	lib, err := LoadSymbolLibrary(strings.NewReader(`<symbols></symbols>`))
	if err != nil {
		t.Fatal(err)
	}
	d := &Diagram{
		Elements: []Element{{ID: "e1", Class: ClassBreaker, Shape: "41", X: 1, Y: 1}},
	}
	var buf bytes.Buffer
	err = Render(d, lib, &buf)
	if err == nil {
		t.Fatal("expected an error for a missing shape")
	}
	if !strings.Contains(err.Error(), "41") {
		t.Errorf("error should name the missing shape: %v", err)
	}
	// The rest of the document must still be written.
	if !strings.Contains(buf.String(), "</svg>") {
		t.Errorf("output should still be a complete document: %s", buf.String())
	}
}
