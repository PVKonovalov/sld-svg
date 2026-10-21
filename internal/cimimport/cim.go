// Package cimimport builds an slddoc.Diagram from a CIM/CGMES RDF/XML
// equipment export plus a React Flow layout JSON that places the same
// equipment on a canvas (e.g. rdss-dms's RTUHVDC_cim-4.xml +
// RTUHVDC_sld-4.json).
//
// The CIM file is the source of truth for what equipment exists, how it is
// connected (Terminal -> ConnectivityNode), its names, switch states, and
// base voltages. It carries no diagram layout at all (no DiagramLayout
// profile), so every position comes from the layout JSON instead. The two
// files share no common identifier for most equipment, so Import joins them
// (see match.go) and skips — and reports — any CIM equipment it cannot place.
package cimimport

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"

// cimObject is one top-level child of rdf:RDF: its class (local name, e.g.
// "Breaker"), its rdf:ID, every literal property by its local name (e.g.
// "IdentifiedObject.name"), and every rdf:resource reference with the
// leading "#" stripped.
type cimObject struct {
	Class string
	ID    string
	Text  map[string]string
	Ref   map[string]string
	// PointIDs are the SCADA point ids named in the object's own
	// "RDSS measurement: ... pointId=<id>" comments — the only per-equipment
	// key the CIM export shares with the layout JSON.
	PointIDs []string
}

func (o *cimObject) name() string { return o.Text["IdentifiedObject.name"] }

// cimModel is a parsed CIM file, objects in document order.
type cimModel struct {
	Objects []*cimObject
	ByID    map[string]*cimObject
}

var pointIDComment = regexp.MustCompile(`pointId=(\S+)`)

// parseCIM reads a CIM RDF/XML document. Only the flat
// object/property/reference structure CIM exports use is understood; nested
// objects are not.
func parseCIM(r io.Reader) (*cimModel, error) {
	dec := xml.NewDecoder(r)
	m := &cimModel{ByID: map[string]*cimObject{}}
	depth := 0
	var cur *cimObject
	var prop string
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cimimport: parsing CIM: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch depth {
			case 2:
				cur = &cimObject{Class: t.Name.Local, Text: map[string]string{}, Ref: map[string]string{}}
				for _, a := range t.Attr {
					if a.Name.Space == rdfNS && (a.Name.Local == "ID" || a.Name.Local == "about") {
						cur.ID = strings.TrimPrefix(a.Value, "#")
					}
				}
			case 3:
				prop = t.Name.Local
				text.Reset()
				for _, a := range t.Attr {
					if a.Name.Space == rdfNS && a.Name.Local == "resource" {
						cur.Ref[prop] = strings.TrimPrefix(a.Value, "#")
					}
				}
			}
		case xml.CharData:
			if depth == 3 {
				text.Write(t)
			}
		case xml.Comment:
			if depth == 2 && cur != nil {
				for _, sm := range pointIDComment.FindAllStringSubmatch(string(t), -1) {
					cur.PointIDs = append(cur.PointIDs, sm[1])
				}
			}
		case xml.EndElement:
			switch depth {
			case 2:
				if cur.ID != "" {
					m.Objects = append(m.Objects, cur)
					m.ByID[cur.ID] = cur
				}
				cur = nil
			case 3:
				if _, isRef := cur.Ref[prop]; !isRef {
					cur.Text[prop] = strings.TrimSpace(text.String())
				}
			}
			depth--
		}
	}
	return m, nil
}

// terminal is one cim:Terminal: the equipment it belongs to and the
// ConnectivityNode it attaches to.
type terminal struct {
	Equipment string
	Node      string
}

// connectivity returns every ConnectivityNode's terminals in document
// order, plus the ConnectivityNode ids themselves in first-seen order.
func (m *cimModel) connectivity() (map[string][]terminal, []string) {
	byNode := map[string][]terminal{}
	var order []string
	for _, o := range m.Objects {
		if o.Class != "Terminal" {
			continue
		}
		t := terminal{Equipment: o.Ref["Terminal.ConductingEquipment"], Node: o.Ref["Terminal.ConnectivityNode"]}
		if t.Equipment == "" || t.Node == "" || strings.EqualFold(o.Text["ACDCTerminal.connected"], "false") {
			continue
		}
		if _, ok := byNode[t.Node]; !ok {
			order = append(order, t.Node)
		}
		byNode[t.Node] = append(byNode[t.Node], t)
	}
	return byNode, order
}

// baseVoltage resolves an equipment object's BaseVoltage id: its own
// ConductingEquipment.BaseVoltage, else its container's VoltageLevel's
// (walking a Bay up to its VoltageLevel).
func (m *cimModel) baseVoltage(o *cimObject) string {
	if bv := o.Ref["ConductingEquipment.BaseVoltage"]; bv != "" {
		return bv
	}
	if bv := o.Ref["TransformerEnd.BaseVoltage"]; bv != "" {
		return bv
	}
	c := m.ByID[o.Ref["Equipment.EquipmentContainer"]]
	for range 3 {
		if c == nil {
			return ""
		}
		if bv := c.Ref["VoltageLevel.BaseVoltage"]; bv != "" {
			return bv
		}
		c = m.ByID[c.Ref["Bay.VoltageLevel"]]
	}
	return ""
}
