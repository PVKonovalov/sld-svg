package cimimport

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/PVKonovalov/slddoc"
)

// Report summarizes what Import did and, above all, what it left out.
type Report struct {
	Elements, Connectors, Nodes, Labels int
	// Unmatched lists CIM equipment no layout node could be found for
	// ("Class id (name)"), in CIM document order. It is not in the diagram.
	Unmatched []string
	// Unsupported counts placed CIM equipment by class that has no slddoc
	// equivalent (e.g. ACDCConverter). It is not drawn either, but wires to
	// it are kept, ending at its layout box.
	Unsupported map[string]int
	// DroppedConnections counts CIM connections (terminal pairs on one
	// ConnectivityNode) not drawn because one side is unmatched.
	DroppedConnections int
	// UnusedLayout counts layout nodes by type that nothing was imported
	// from (group boxes, and equipment no CIM object matched).
	UnusedLayout map[string]int
}

type point struct{ X, Y float64 }

// shapeSpec is how one CIM class is drawn: its slddoc class and shape. The
// shape's terminals come from the element library (base.xml's
// <terminals>, which Repair also fits ports to). base is the extra rotation
// from the shape's local frame to the layout icon's own unrotated frame —
// e.g. the layout draws a transformer upright while shape 47's windings lie
// along x. anchorTop anchors the element at the icon's top edge instead of
// its center, for a shape whose anchor is its terminal.
type shapeSpec struct {
	class     slddoc.Class
	shape     string
	terminals []point
	base      int
	anchorTop bool
	switching bool
}

var (
	specBreaker      = shapeSpec{class: slddoc.ClassBreaker, shape: "41", switching: true}
	specDisconnector = shapeSpec{class: slddoc.ClassDisconnector, shape: "162", switching: true}
	// 54's terminal is at its bottom, the layout's earth switch connects at its top.
	specGroundSwitch       = shapeSpec{class: slddoc.ClassGroundSwitch, shape: "54", base: 180, switching: true}
	specGround             = shapeSpec{class: slddoc.ClassGround, shape: "31"}
	specCapacitor          = shapeSpec{class: slddoc.ClassCapacitor, shape: "388"}
	specReactor            = shapeSpec{class: slddoc.ClassReactor, shape: "37"}
	specVoltageTransformer = shapeSpec{class: slddoc.ClassVoltageTransformer, shape: "55", anchorTop: true}
	specSurgeArrester      = shapeSpec{class: slddoc.ClassSurgeArrester, shape: "35"}
	specPowerTransformer   = shapeSpec{class: slddoc.ClassPowerTransformer, shape: "47", base: 90}
	specBusbar             = shapeSpec{class: slddoc.ClassBusBarSection, shape: "24"}

	// deviceSpecs are every spec placed by its library terminals (all but
	// the busbar, which takes a port wherever something meets it).
	deviceSpecs = []shapeSpec{specBreaker, specDisconnector, specGroundSwitch, specGround, specCapacitor,
		specReactor, specVoltageTransformer, specSurgeArrester, specPowerTransformer}
)

// specFor picks the drawing for a CIM object. Shunt compensators are drawn
// as reactors or capacitors by the layout icon, since CIM's
// (Linear)ShuntCompensator covers both.
func specFor(o *cimObject, n *layoutNode) (shapeSpec, bool) {
	switch o.Class {
	case "Breaker":
		return specBreaker, true
	case "Switch", "Disconnector":
		return specDisconnector, true
	case "GroundDisconnector":
		return specGroundSwitch, true
	case "Ground":
		return specGround, true
	case "ShuntCompensator", "LinearShuntCompensator", "FilterBank":
		if n.Type == "inductor" || n.Type == "reactor" {
			return specReactor, true
		}
		return specCapacitor, true
	case "PotentialTransformer":
		return specVoltageTransformer, true
	case "SurgeArrester":
		return specSurgeArrester, true
	case "PowerTransformer":
		return specPowerTransformer, true
	case "BusbarSection":
		return specBusbar, true
	}
	return shapeSpec{}, false
}

// voltagePalette colors voltage classes in order of first use; CIM carries
// no display color.
var voltagePalette = []string{"#00A0F0", "#962896", "#FFD700", "#00C000", "#FF8000", "#F05050", "#C0C0C0"}

const (
	margin           = 40
	defaultLabelSize = 11
)

// placedEq is one CIM equipment object with a layout position.
type placedEq struct {
	obj       *cimObject
	node      *layoutNode
	box       box
	spec      shapeSpec
	supported bool
	elIndex   int
	center    point
	orient    int
	ports     []point // absolute, one per spec terminal
	voltages  []int   // voltage class per port
	voltage   int
	busFrom   point
	busTo     point
}

// endpoint is one wire end: a key identifying the electrical node it lands
// on (an element port, a busbar attachment, or a free end at an
// unsupported device's box) and its position.
type endpoint struct {
	key     string
	pos     point
	voltage int
	// dir is the direction the wire leaves the endpoint in: vertical
	// (true) or horizontal, used to route it.
	vertical bool
}

// Import reads a CIM RDF/XML export and the React Flow layout JSON placing
// its equipment, and builds a Diagram from them, with every device's ports
// on its shape's terminals (LoadTerminals). The result is then repaired the
// way sld-editor repairs an imported diagram (slddoc.Repair, then
// slddoc.EnsureIDs for the ids Repair added). source is recorded as the
// diagram's Source.
func Import(cimFile, layoutFile io.Reader, terminals map[string][]slddoc.Point, source string) (*slddoc.Diagram, Report, error) {
	terminals = withDerivedTerminals(terminals)
	var missing []string
	for _, s := range deviceSpecs {
		if len(terminals[s.shape]) == 0 {
			missing = append(missing, s.shape)
		}
	}
	if len(missing) > 0 {
		return nil, Report{}, fmt.Errorf("cimimport: element library has no terminals for shape(s) %s", strings.Join(missing, ", "))
	}
	m, err := parseCIM(cimFile)
	if err != nil {
		return nil, Report{}, err
	}
	layout, err := parseLayout(layoutFile)
	if err != nil {
		return nil, Report{}, err
	}
	b := newBuilder(m, layout, terminals, source)
	b.build()
	slddoc.Repair(b.d, terminals)
	slddoc.EnsureIDs(b.d)
	b.report.Elements = len(b.d.Elements)
	b.report.Connectors = len(b.d.Connectors)
	b.report.Nodes = len(b.d.Nodes)
	b.report.Labels = len(b.d.Labels)
	return b.d, b.report, nil
}

type builder struct {
	m         *cimModel
	terminals map[string][]slddoc.Point
	layout    *layoutDoc
	d         *slddoc.Diagram
	report    Report

	nextID  int
	shift   point
	nodes   map[string]*layoutNode
	placed  map[string]*placedEq
	order   []*placedEq
	voltage map[string]int // BaseVoltage id -> VoltageClass id

	// endpoint keys, union-found where a connection's two ends coincide
	keyPos    map[string]point
	keyOrder  []string
	parent    map[string]string
	usedEdges map[int]bool
	pending   []pendingWire
}

func newBuilder(m *cimModel, layout *layoutDoc, terminals map[string][]slddoc.Point, source string) *builder {
	return &builder{
		m:         m,
		terminals: terminals,
		layout:    layout,
		d: &slddoc.Diagram{
			Source: source,
			Layers: []slddoc.Layer{{ID: slddoc.BaseLayer, Name: "Base"}},
		},
		report:    Report{Unsupported: map[string]int{}, UnusedLayout: map[string]int{}},
		nodes:     map[string]*layoutNode{},
		placed:    map[string]*placedEq{},
		voltage:   map[string]int{},
		keyPos:    map[string]point{},
		parent:    map[string]string{},
		usedEdges: map[int]bool{},
	}
}

func (b *builder) id() int {
	b.nextID++
	return b.nextID
}

func (b *builder) build() {
	// Shift the layout so its top-left lands at the margin, on the 10-grid.
	minX, minY := math.Inf(1), math.Inf(1)
	for i := range b.layout.Nodes {
		n := &b.layout.Nodes[i]
		b.nodes[n.ID] = n
		bx := n.box()
		minX, minY = math.Min(minX, bx.X), math.Min(minY, bx.Y)
	}
	if len(b.layout.Nodes) > 0 {
		b.shift = point{margin - math.Floor(minX/10)*10, margin - math.Floor(minY/10)*10}
	}

	cn, cnOrder := b.m.connectivity()
	hasTerminal := map[string]bool{}
	for _, ts := range cn {
		for _, t := range ts {
			hasTerminal[t.Equipment] = true
		}
	}
	var equipment []*cimObject
	for _, o := range b.m.Objects {
		if hasTerminal[o.ID] && o.Class != "Terminal" {
			equipment = append(equipment, o)
		}
	}

	match := matchEquipment(b.m, equipment, b.layout, cn)
	matchedNode := map[string]bool{}
	for _, o := range equipment {
		nodeID, ok := match[o.ID]
		if !ok {
			b.report.Unmatched = append(b.report.Unmatched, fmt.Sprintf("%s %s (%s)", o.Class, o.ID, o.name()))
			continue
		}
		matchedNode[nodeID] = true
		b.place(o, b.nodes[nodeID])
	}

	for _, cnID := range cnOrder {
		b.connect(cn[cnID])
	}
	b.finish()

	for i := range b.layout.Nodes {
		n := &b.layout.Nodes[i]
		switch {
		case n.Type == "text_label":
			b.addLabel(n)
		case !matchedNode[n.ID]:
			b.report.UnusedLayout[n.Type]++
		}
	}

	b.d.LastID = b.nextID
}

func (b *builder) shifted(n *layoutNode) box {
	bx := n.box()
	bx.X += b.shift.X
	bx.Y += b.shift.Y
	return bx
}

func (b *builder) voltageClass(o *cimObject) int {
	bvID := b.m.baseVoltage(o)
	if bvID == "" {
		return 0
	}
	if id, ok := b.voltage[bvID]; ok {
		return id
	}
	name := bvID
	if bv := b.m.ByID[bvID]; bv != nil && bv.name() != "" {
		name = bv.name()
	}
	id := b.id()
	b.voltage[bvID] = id
	b.d.VoltageClasses = append(b.d.VoltageClasses, slddoc.VoltageClass{
		ID: id, Name: name, Color: voltagePalette[(len(b.d.VoltageClasses))%len(voltagePalette)],
	})
	return id
}

// place positions one matched CIM object from its layout node and, when its
// class is supported, adds its Element (ports are filled in by finish).
func (b *builder) place(o *cimObject, n *layoutNode) {
	p := &placedEq{obj: o, node: n, box: b.shifted(n), elIndex: -1}
	b.placed[o.ID] = p
	spec, ok := specFor(o, n)
	if !ok {
		b.report.Unsupported[o.Class]++
		return
	}
	for _, t := range b.terminals[spec.shape] {
		spec.terminals = append(spec.terminals, point{t.X, t.Y})
	}
	p.spec, p.supported = spec, true
	p.voltage = b.voltageClass(o)
	el := slddoc.Element{
		ID:      b.id(),
		Class:   spec.class,
		Shape:   spec.shape,
		Name:    o.name(),
		Voltage: p.voltage,
		Layer:   slddoc.BaseLayer,
	}

	if spec.class == slddoc.ClassBusBarSection {
		c := p.box.center()
		if p.box.W >= p.box.H {
			p.busFrom, p.busTo = point{p.box.X, c.Y}, point{p.box.X + p.box.W, c.Y}
		} else {
			p.busFrom, p.busTo = point{c.X, p.box.Y}, point{c.X, p.box.Y + p.box.H}
		}
		p.busFrom, p.busTo = round(p.busFrom), round(p.busTo)
		el.X, el.Y = p.busFrom.X, p.busFrom.Y
		el.Points = []slddoc.Point{{X: p.busFrom.X, Y: p.busFrom.Y}, {X: p.busTo.X, Y: p.busTo.Y}}
	} else {
		rot := n.Data.Rotation
		p.orient = normalizeAngle(spec.base + rot)
		p.center = p.box.center()
		if spec.anchorTop {
			p.center = add(p.center, rotate(point{0, -p.box.H / 2}, rot))
		}
		p.center = round(p.center)
		el.X, el.Y, el.Orient = p.center.X, p.center.Y, p.orient
		for i, t := range spec.terminals {
			pos := round(add(p.center, rotate(t, p.orient)))
			p.ports = append(p.ports, pos)
			p.voltages = append(p.voltages, p.voltage)
			b.register(portKey(o.ID, i), pos)
		}
	}
	if spec.switching {
		if open, ok := o.Text["Switch.open"]; ok {
			state := 1
			if strings.EqualFold(open, "true") {
				state = 0
			}
			el.State = &state
		}
	}
	if spec.class == slddoc.ClassPowerTransformer {
		el.Windings = b.windings(o)
		for i := range p.voltages {
			if i < len(el.Windings) && el.Windings[i].Voltage != 0 {
				p.voltages[i] = el.Windings[i].Voltage
			}
		}
		if len(el.Windings) > 0 && el.Windings[0].Voltage != 0 {
			el.Voltage = el.Windings[0].Voltage
		}
	}
	p.elIndex = len(b.d.Elements)
	b.d.Elements = append(b.d.Elements, el)
	b.order = append(b.order, p)
}

// windings returns a transformer's first two PowerTransformerEnds' voltages,
// by endNumber (shape 47 draws the 2-winding case only).
func (b *builder) windings(o *cimObject) []slddoc.TransformerWinding {
	var ends []*cimObject
	for _, e := range b.m.Objects {
		if e.Class == "PowerTransformerEnd" && e.Ref["PowerTransformerEnd.PowerTransformer"] == o.ID {
			ends = append(ends, e)
		}
	}
	sort.SliceStable(ends, func(i, j int) bool {
		a, _ := strconv.Atoi(ends[i].Text["TransformerEnd.endNumber"])
		c, _ := strconv.Atoi(ends[j].Text["TransformerEnd.endNumber"])
		return a < c
	})
	ws := make([]slddoc.TransformerWinding, 2)
	for i := 0; i < len(ends) && i < 2; i++ {
		ws[i].Voltage = b.voltageClass(ends[i])
	}
	return ws
}

// withDerivedTerminals returns terminals plus shape 47's: a power
// transformer has no library <terminals>, slddoc derives them from its
// windings instead. They are read back from slddoc.Repair itself, which
// gives a port-less 2-winding transformer at the origin a Node on each
// terminal, so they always match what Repair later fits ports to. The
// caller's map is not modified.
func withDerivedTerminals(terminals map[string][]slddoc.Point) map[string][]slddoc.Point {
	shape := specPowerTransformer.shape
	if len(terminals[shape]) > 0 {
		return terminals
	}
	probe := &slddoc.Diagram{
		Layers: []slddoc.Layer{{ID: slddoc.BaseLayer}},
		Elements: []slddoc.Element{{
			ID: 1, Class: specPowerTransformer.class, Shape: shape,
			Windings: make([]slddoc.TransformerWinding, 2),
		}},
		LastID: 1,
	}
	slddoc.Repair(probe, terminals)
	pos := map[int]slddoc.Point{}
	for _, n := range probe.Nodes {
		pos[n.ID] = slddoc.Point{X: n.X, Y: n.Y}
	}
	var derived []slddoc.Point
	for _, port := range probe.Elements[0].Ports {
		if p, ok := pos[port.Node]; ok {
			derived = append(derived, p)
		}
	}
	out := make(map[string][]slddoc.Point, len(terminals)+1)
	for k, v := range terminals {
		out[k] = v
	}
	out[shape] = derived
	return out
}

func portKey(eqID string, i int) string { return fmt.Sprintf("%s#%d", eqID, i) }

func (b *builder) register(key string, pos point) {
	if _, ok := b.keyPos[key]; ok {
		return
	}
	b.keyPos[key] = pos
	b.keyOrder = append(b.keyOrder, key)
	b.parent[key] = key
}

func (b *builder) find(k string) string {
	for b.parent[k] != k {
		b.parent[k] = b.parent[b.parent[k]]
		k = b.parent[k]
	}
	return k
}

// connect draws one ConnectivityNode's connections. Two terminals are one
// wire, paired with the layout edge between their nodes for its handles;
// more are drawn as a star from a busbar terminal, if any, else the first.
func (b *builder) connect(ts []terminal) {
	if len(ts) < 2 {
		return
	}
	hub := 0
	for i, t := range ts {
		if p := b.placed[t.Equipment]; p != nil && p.spec.class == slddoc.ClassBusBarSection {
			hub = i
			break
		}
	}
	for i, t := range ts {
		if i == hub {
			continue
		}
		b.wire(ts[hub].Equipment, t.Equipment, len(ts) == 2)
	}
}

func (b *builder) wire(aID, bID string, useEdge bool) {
	pa, pb := b.placed[aID], b.placed[bID]
	if pa == nil || pb == nil {
		b.report.DroppedConnections++
		return
	}
	var ha, hb string
	if useEdge {
		ha, hb = b.edgeHandles(pa.node.ID, pb.node.ID)
	}
	// A busbar end attaches where the other end lands, so resolve the other
	// end first.
	var ea, eb endpoint
	if pa.spec.class == slddoc.ClassBusBarSection && pb.spec.class != slddoc.ClassBusBarSection {
		eb = b.endpoint(pb, hb, pa.box.center())
		ea = b.endpoint(pa, ha, eb.pos)
	} else {
		ea = b.endpoint(pa, ha, pb.box.center())
		eb = b.endpoint(pb, hb, ea.pos)
	}

	if ea.pos == eb.pos {
		b.parent[b.find(eb.key)] = b.find(ea.key)
		return
	}
	v := ea.voltage
	if v == 0 {
		v = eb.voltage
	}
	b.pending = append(b.pending, pendingWire{from: ea.key, to: eb.key, points: route(ea, eb), voltage: v})
}

// edgeHandles returns the handles of the first not-yet-used layout edge
// between two nodes, oriented as (a, b).
func (b *builder) edgeHandles(a, c string) (string, string) {
	for i, e := range b.layout.Edges {
		if b.usedEdges[i] {
			continue
		}
		switch {
		case e.Source == a && e.Target == c:
			b.usedEdges[i] = true
			return e.SourceHandle, e.TargetHandle
		case e.Source == c && e.Target == a:
			b.usedEdges[i] = true
			return e.TargetHandle, e.SourceHandle
		}
	}
	return "", ""
}

var handleDir = map[string]point{"t": {0, -1}, "b": {0, 1}, "l": {-1, 0}, "r": {1, 0}}

// endpoint resolves where a wire attaching to p by the given handle lands.
// ref is the wire's other end (or the other device), used to pick between
// equally good ports and to place a busbar attachment.
func (b *builder) endpoint(p *placedEq, handle string, ref point) endpoint {
	switch {
	case !p.supported:
		pos := round(p.box.handlePoint(handle))
		key := fmt.Sprintf("%s!%v", p.obj.ID, pos)
		b.register(key, pos)
		return endpoint{key: key, pos: pos, vertical: handle != "l" && handle != "r"}
	case p.spec.class == slddoc.ClassBusBarSection:
		var pos point
		switch handle {
		case "l":
			pos = p.busFrom
		case "r":
			pos = p.busTo
		default:
			pos = round(projectOnSegment(ref, p.busFrom, p.busTo))
		}
		key := fmt.Sprintf("%s@%v", p.obj.ID, pos)
		b.register(key, pos)
		busHorizontal := p.busFrom.Y == p.busTo.Y
		return endpoint{key: key, pos: pos, voltage: p.voltage, vertical: busHorizontal}
	}
	i := p.portFor(handle, ref)
	dir := sub(p.ports[i], p.center)
	return endpoint{
		key:      portKey(p.obj.ID, i),
		pos:      p.ports[i],
		voltage:  p.voltages[i],
		vertical: math.Abs(dir.Y) >= math.Abs(dir.X),
	}
}

// portFor picks the port a handle names: the terminal pointing the most in
// the handle's direction, in the layout icon's own frame; ties, and handles
// with no direction, go to the port nearest ref.
func (p *placedEq) portFor(handle string, ref point) int {
	if len(p.ports) == 1 {
		return 0
	}
	hd, hasDir := handleDir[handle]
	best, bestScore, bestDist := -1, 0.0, 0.0
	for i, t := range p.spec.terminals {
		score := 0.0
		if hasDir {
			d := rotate(t, p.spec.base)
			if l := math.Hypot(d.X, d.Y); l > 0 {
				score = (d.X*hd.X + d.Y*hd.Y) / l
			}
		}
		dist := math.Hypot(p.ports[i].X-ref.X, p.ports[i].Y-ref.Y)
		if best < 0 || score > bestScore+1e-9 || (math.Abs(score-bestScore) <= 1e-9 && dist < bestDist) {
			best, bestScore, bestDist = i, score, dist
		}
	}
	return best
}

type pendingWire struct {
	from, to string
	points   []point
	voltage  int
}

// finish allocates Nodes for every endpoint key, fills in element ports,
// and writes the wires.
func (b *builder) finish() {
	nodeOf := map[string]int{}
	for _, k := range b.keyOrder {
		root := b.find(k)
		if _, ok := nodeOf[root]; !ok {
			pos := b.keyPos[root]
			id := b.id()
			nodeOf[root] = id
			b.d.Nodes = append(b.d.Nodes, slddoc.Node{ID: id, X: pos.X, Y: pos.Y})
		}
		nodeOf[k] = nodeOf[root]
	}

	for _, p := range b.order {
		el := &b.d.Elements[p.elIndex]
		if p.spec.class == slddoc.ClassBusBarSection {
			seen := map[int]bool{}
			for _, k := range b.keyOrder {
				if strings.HasPrefix(k, p.obj.ID+"@") && !seen[nodeOf[k]] {
					seen[nodeOf[k]] = true
					el.Ports = append(el.Ports, slddoc.Port{Name: strconv.Itoa(len(el.Ports) + 1), Node: nodeOf[k]})
				}
			}
			continue
		}
		for i := range p.ports {
			el.Ports = append(el.Ports, slddoc.Port{Name: strconv.Itoa(i + 1), Node: nodeOf[portKey(p.obj.ID, i)]})
		}
	}

	for _, w := range b.pending {
		from, to := nodeOf[w.from], nodeOf[w.to]
		if from == to {
			continue
		}
		c := slddoc.Connector{
			ID:      b.id(),
			Kind:    slddoc.KindBusbarWire,
			Voltage: w.voltage,
			Layer:   slddoc.BaseLayer,
			From:    from,
			To:      to,
		}
		for _, pt := range w.points {
			c.Points = append(c.Points, slddoc.Point{X: pt.X, Y: pt.Y})
		}
		b.d.Connectors = append(b.d.Connectors, c)
	}

	maxX, maxY := 0.0, 0.0
	for i := range b.layout.Nodes {
		bx := b.shifted(&b.layout.Nodes[i])
		maxX, maxY = math.Max(maxX, bx.X+bx.W), math.Max(maxY, bx.Y+bx.H)
	}
	b.d.Width, b.d.Height = math.Ceil(maxX)+margin, math.Ceil(maxY)+margin
}

func (b *builder) addLabel(n *layoutNode) {
	text := strings.TrimSpace(n.Data.Text)
	if text == "" {
		return
	}
	c := round(b.shifted(n).center())
	size := n.Data.FontSize
	if size == 0 {
		size = defaultLabelSize
	}
	b.d.Labels = append(b.d.Labels, slddoc.Label{
		ID:     b.id(),
		Layer:  slddoc.BaseLayer,
		X:      c.X,
		Y:      c.Y,
		Size:   size,
		Anchor: "middle",
		VAlign: "middle",
		Bold:   n.Data.Bold,
		Color:  n.Data.Color,
		Text:   text,
	})
}

// route draws a rectilinear "step" wire, leaving the first end along its
// own direction.
func route(a, c endpoint) []point {
	p, q := a.pos, c.pos
	switch {
	case p.X == q.X || p.Y == q.Y:
		return []point{p, q}
	case a.vertical:
		my := round(point{0, (p.Y + q.Y) / 2}).Y
		return []point{p, {p.X, my}, {q.X, my}, q}
	default:
		mx := round(point{(p.X + q.X) / 2, 0}).X
		return []point{p, {mx, p.Y}, {mx, q.Y}, q}
	}
}

// rotate turns v clockwise by deg degrees, SVG's rotate() convention.
func rotate(v point, deg int) point {
	r := float64(deg) * math.Pi / 180
	s, c := math.Sin(r), math.Cos(r)
	return point{v.X*c - v.Y*s, v.X*s + v.Y*c}
}

func normalizeAngle(deg int) int {
	deg %= 360
	if deg > 180 {
		deg -= 360
	} else if deg <= -180 {
		deg += 360
	}
	return deg
}

func add(a, c point) point { return point{a.X + c.X, a.Y + c.Y} }
func sub(a, c point) point { return point{a.X - c.X, a.Y - c.Y} }

// round canonicalizes rotation noise away (coordinates stay on a 0.01 grid).
func round(p point) point {
	return point{math.Round(p.X*100) / 100, math.Round(p.Y*100) / 100}
}

func projectOnSegment(p, a, c point) point {
	d := sub(c, a)
	l2 := d.X*d.X + d.Y*d.Y
	if l2 == 0 {
		return a
	}
	t := ((p.X-a.X)*d.X + (p.Y-a.Y)*d.Y) / l2
	t = math.Max(0, math.Min(1, t))
	return point{a.X + t*d.X, a.Y + t*d.Y}
}
