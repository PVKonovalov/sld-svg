package cimimport

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/PVKonovalov/slddoc"
)

// LoadTerminals reads each shape's local, unrotated <terminals> from an
// element library in sld-editor's base.xml format — the same map sld-editor
// hands slddoc.Repair. A shape with no <terminals> (a busbar) is left out.
func LoadTerminals(r io.Reader) (map[string][]slddoc.Point, error) {
	var raw struct {
		Symbols []struct {
			Shape     string         `xml:"shape,attr"`
			Terminals []slddoc.Point `xml:"terminals>terminal"`
		} `xml:"symbol"`
	}
	if err := xml.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("cimimport: parsing element library: %w", err)
	}
	out := map[string][]slddoc.Point{}
	for _, s := range raw.Symbols {
		if len(s.Terminals) > 0 {
			out[s.Shape] = s.Terminals
		}
	}
	return out, nil
}
