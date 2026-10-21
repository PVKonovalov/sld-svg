package cimimport

import (
	"strings"
	"testing"

	"github.com/PVKonovalov/slddoc"
)

// fixtureCIM is adapted from rdss-dms's RTUHVDC_cim-4.xml: a busbar (placed
// via its BAY_<layout id> container), a breaker (placed via its measurement
// pointId), a ground switch below it (placed by topology propagation), and
// a source with no layout counterpart of its own class (skipped).
const fixtureCIM = `<?xml version="1.0" encoding="UTF-8"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:cim="http://iec.ch/TC57/2013/CIM-schema-cim16#">
  <cim:BaseVoltage rdf:ID="BV_11kV">
    <cim:IdentifiedObject.name>11kV</cim:IdentifiedObject.name>
  </cim:BaseVoltage>
  <cim:VoltageLevel rdf:ID="VL_11">
    <cim:VoltageLevel.BaseVoltage rdf:resource="#BV_11kV"/>
  </cim:VoltageLevel>
  <cim:Bay rdf:ID="BAY_busbar-1">
    <cim:IdentifiedObject.name>BAY_busbar-1</cim:IdentifiedObject.name>
    <cim:Bay.VoltageLevel rdf:resource="#VL_11"/>
  </cim:Bay>
  <cim:BusbarSection rdf:ID="BB1">
    <cim:IdentifiedObject.name>Bus 1</cim:IdentifiedObject.name>
    <cim:Equipment.EquipmentContainer rdf:resource="#BAY_busbar-1"/>
  </cim:BusbarSection>
  <cim:Breaker rdf:ID="CB1">
    <cim:IdentifiedObject.name>Q50</cim:IdentifiedObject.name>
    <cim:Equipment.EquipmentContainer rdf:resource="#VL_11"/>
    <cim:Switch.open>true</cim:Switch.open>
    <!-- RDSS measurement: role=cb_state pointId=RTU-Q50.cb_5050 -->
  </cim:Breaker>
  <cim:GroundDisconnector rdf:ID="ES1">
    <cim:IdentifiedObject.name>Q51</cim:IdentifiedObject.name>
    <cim:Equipment.EquipmentContainer rdf:resource="#VL_11"/>
    <cim:Switch.open>false</cim:Switch.open>
  </cim:GroundDisconnector>
  <cim:EnergySource rdf:ID="SRC1">
    <cim:IdentifiedObject.name>Incoming Feeder</cim:IdentifiedObject.name>
  </cim:EnergySource>
  <cim:ConnectivityNode rdf:ID="CN1"/>
  <cim:ConnectivityNode rdf:ID="CN2"/>
  <cim:ConnectivityNode rdf:ID="CN3"/>
  <cim:Terminal rdf:ID="T1"><cim:Terminal.ConductingEquipment rdf:resource="#BB1"/><cim:Terminal.ConnectivityNode rdf:resource="#CN1"/></cim:Terminal>
  <cim:Terminal rdf:ID="T2"><cim:Terminal.ConductingEquipment rdf:resource="#CB1"/><cim:Terminal.ConnectivityNode rdf:resource="#CN1"/></cim:Terminal>
  <cim:Terminal rdf:ID="T3"><cim:Terminal.ConductingEquipment rdf:resource="#CB1"/><cim:Terminal.ConnectivityNode rdf:resource="#CN2"/></cim:Terminal>
  <cim:Terminal rdf:ID="T4"><cim:Terminal.ConductingEquipment rdf:resource="#ES1"/><cim:Terminal.ConnectivityNode rdf:resource="#CN2"/></cim:Terminal>
  <cim:Terminal rdf:ID="T5"><cim:Terminal.ConductingEquipment rdf:resource="#SRC1"/><cim:Terminal.ConnectivityNode rdf:resource="#CN3"/></cim:Terminal>
  <cim:Terminal rdf:ID="T6"><cim:Terminal.ConductingEquipment rdf:resource="#BB1"/><cim:Terminal.ConnectivityNode rdf:resource="#CN3"/></cim:Terminal>
</rdf:RDF>`

const fixtureLayout = `{
  "nodes": [
    {"id": "busbar-1", "type": "busbar", "position": {"x": 0, "y": 0}, "style": {"width": 200, "height": 4},
     "data": {"cimClass": "cim:BusbarSection"}},
    {"id": "breaker-1", "type": "breaker", "position": {"x": 88, "y": 50}, "style": {"width": 24, "height": 24},
     "data": {"cimClass": "cim:Breaker", "liveValues": {"RTU-Q50.cb_5050": "1"}}},
    {"id": "earth-1", "type": "earth_switch", "position": {"x": 88, "y": 150}, "style": {"width": 24, "height": 28},
     "data": {"cimClass": "cim:GroundDisconnector"}},
    {"id": "incoming-1", "type": "incoming", "position": {"x": 300, "y": 0}, "style": {"width": 24, "height": 32},
     "data": {"cimClass": "cim:ACLineSegment"}},
    {"id": "label-1", "type": "text_label", "position": {"x": 0, "y": -30}, "style": {"width": 80, "height": 24},
     "data": {"cimClass": "cim:ConductingEquipment", "text": "WEST BUS", "fontSize": 12, "color": "#185FA5"}}
  ],
  "edges": [
    {"source": "busbar-1", "sourceHandle": "t2", "target": "breaker-1", "targetHandle": "t"},
    {"source": "breaker-1", "sourceHandle": "b", "target": "earth-1", "targetHandle": "t"},
    {"source": "incoming-1", "sourceHandle": "b", "target": "busbar-1", "targetHandle": "r"}
  ]
}`

// fixtureLibrary carries base.xml's <terminals> for every shape Import
// places, and nothing else. Like base.xml it has none for 47: a power
// transformer's come from slddoc (withDerivedTerminals).
const fixtureLibrary = `<symbols>
  <symbol shape="41"><terminals><terminal x="0" y="-10"/><terminal x="0" y="10"/></terminals></symbol>
  <symbol shape="162"><terminals><terminal x="0" y="-10"/><terminal x="0" y="10"/></terminals></symbol>
  <symbol shape="54"><terminals><terminal x="0" y="10"/></terminals></symbol>
  <symbol shape="31"><terminals><terminal x="0" y="-10"/></terminals></symbol>
  <symbol shape="388"><terminals><terminal x="0" y="-10"/><terminal x="0" y="10"/></terminals></symbol>
  <symbol shape="37"><terminals><terminal x="0" y="-20"/><terminal x="0" y="20"/></terminals></symbol>
  <symbol shape="55"><terminals><terminal x="0" y="0"/></terminals></symbol>
  <symbol shape="35"><terminals><terminal x="0" y="-10"/><terminal x="0" y="10"/></terminals></symbol>
  <symbol shape="24"><template>busbar, no terminals</template></symbol>
</symbols>`

func fixtureTerminals(t *testing.T) map[string][]slddoc.Point {
	t.Helper()
	terminals, err := LoadTerminals(strings.NewReader(fixtureLibrary))
	if err != nil {
		t.Fatal(err)
	}
	return terminals
}

func importFixture(t *testing.T) (*slddoc.Diagram, Report) {
	t.Helper()
	d, r, err := Import(strings.NewReader(fixtureCIM), strings.NewReader(fixtureLayout), fixtureTerminals(t), "fixture.xml")
	if err != nil {
		t.Fatal(err)
	}
	return d, r
}

func element(t *testing.T, d *slddoc.Diagram, name string) slddoc.Element {
	t.Helper()
	for _, e := range d.Elements {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no element %q", name)
	return slddoc.Element{}
}

func nodePos(d *slddoc.Diagram, id int) point {
	for _, n := range d.Nodes {
		if n.ID == id {
			return point{n.X, n.Y}
		}
	}
	return point{-1, -1}
}

func TestImport_MatchesAndSkips(t *testing.T) {
	d, r := importFixture(t)
	if len(d.Elements) != 3 {
		t.Fatalf("got %d elements, want 3 (bus, breaker, ground switch)", len(d.Elements))
	}
	if len(r.Unmatched) != 1 || !strings.HasPrefix(r.Unmatched[0], "EnergySource SRC1") {
		t.Errorf("Unmatched = %v, want only the EnergySource", r.Unmatched)
	}
	if r.DroppedConnections != 1 {
		t.Errorf("DroppedConnections = %d, want 1 (the source's bus tie)", r.DroppedConnections)
	}
	if r.UnusedLayout["incoming"] != 1 {
		t.Errorf("UnusedLayout = %v, want the incoming node", r.UnusedLayout)
	}
	if len(d.Labels) != 1 || d.Labels[0].Text != "WEST BUS" {
		t.Errorf("Labels = %+v", d.Labels)
	}
}

func TestImport_StatesAndVoltage(t *testing.T) {
	d, _ := importFixture(t)
	cb, es := element(t, d, "Q50"), element(t, d, "Q51")
	if cb.State == nil || *cb.State != 0 {
		t.Errorf("breaker state = %v, want 0 (Switch.open=true)", cb.State)
	}
	if es.State == nil || *es.State != 1 {
		t.Errorf("ground switch state = %v, want 1", es.State)
	}
	if len(d.VoltageClasses) != 1 || d.VoltageClasses[0].Name != "11kV" {
		t.Fatalf("VoltageClasses = %+v", d.VoltageClasses)
	}
	bus := element(t, d, "Bus 1")
	if bus.Voltage != d.VoltageClasses[0].ID {
		t.Errorf("bus voltage %d not resolved through its Bay", bus.Voltage)
	}
}

func TestImport_Geometry(t *testing.T) {
	d, _ := importFixture(t)
	cb, es, bus := element(t, d, "Q50"), element(t, d, "Q51"), element(t, d, "Bus 1")

	// The ground switch's shape-54 terminal sits at its bottom, so it is
	// turned to face the breaker above it.
	if es.Orient != 180 {
		t.Errorf("ground switch orient = %d, want 180", es.Orient)
	}
	top, bottom := nodePos(d, cb.Ports[0].Node), nodePos(d, cb.Ports[1].Node)
	if top.Y != cb.Y-10 || bottom.Y != cb.Y+10 || top.X != cb.X {
		t.Errorf("breaker ports at %v/%v, want (x, y∓10) around %v,%v", top, bottom, cb.X, cb.Y)
	}
	if len(bus.Ports) != 1 {
		t.Fatalf("bus ports = %+v, want one attachment", bus.Ports)
	}
	if at := nodePos(d, bus.Ports[0].Node); at.X != cb.X || at.Y != bus.Y {
		t.Errorf("bus attachment at %v, want straight above the breaker (%v,%v)", at, cb.X, bus.Y)
	}
	if len(d.Connectors) != 2 {
		t.Fatalf("got %d connectors, want 2", len(d.Connectors))
	}
	for _, c := range d.Connectors {
		first, last := c.Points[0], c.Points[len(c.Points)-1]
		if (point{first.X, first.Y}) != nodePos(d, c.From) || (point{last.X, last.Y}) != nodePos(d, c.To) {
			t.Errorf("connector %d does not run between its nodes: %+v", c.ID, c)
		}
	}
}

func TestLoadTerminals(t *testing.T) {
	terminals := fixtureTerminals(t)
	if got := terminals["54"]; len(got) != 1 || got[0] != (slddoc.Point{X: 0, Y: 10}) {
		t.Errorf(`terminals["54"] = %v`, got)
	}
	if _, ok := terminals["24"]; ok {
		t.Error("a shape with no <terminals> must be left out")
	}
}

func TestImport_MissingTerminals(t *testing.T) {
	terminals := fixtureTerminals(t)
	delete(terminals, "41")
	_, _, err := Import(strings.NewReader(fixtureCIM), strings.NewReader(fixtureLayout), terminals, "fixture.xml")
	if err == nil || !strings.Contains(err.Error(), "41") {
		t.Errorf("err = %v, want one naming shape 41", err)
	}
}

func TestWithDerivedTerminals(t *testing.T) {
	terminals := fixtureTerminals(t)
	got := withDerivedTerminals(terminals)["47"]
	if len(got) != 2 || got[0].Y != 0 || got[1].Y != 0 || got[0].X != -got[1].X || got[0].X == 0 {
		t.Errorf("transformer terminals = %v, want a symmetric pair on the x axis", got)
	}
	if _, ok := terminals["47"]; ok {
		t.Error("withDerivedTerminals modified its argument")
	}
}
