package cimimport

import "strings"

// matchEquipment pairs CIM equipment with layout nodes, returning CIM id ->
// layout node id. A pair always requires the layout node's own
// data.cimClass to name the CIM object's class, and no layout node is ever
// used twice. Matching runs in three steps:
//
//  1. Measurement seeds: a CIM object's "RDSS measurement ... pointId=" ids
//     must all appear on exactly one layout node (data.points / liveValues).
//  2. Busbar seeds: a BusbarSection's container Bay is named
//     "BAY_<layout node id>" — the one place the export embeds a layout id.
//  3. Propagation through topology: an unmatched object whose matched CIM
//     neighbors (equipment sharing a ConnectivityNode with it) all point at
//     layout nodes with exactly one common, unused, same-class edge
//     neighbor is matched to it; repeated until nothing changes.
//
// Anything still ambiguous after that (e.g. two identical earth switches
// hanging off the same point) stays unmatched rather than being guessed.
func matchEquipment(m *cimModel, equipment []*cimObject, layout *layoutDoc, cnTerminals map[string][]terminal) map[string]string {
	nodes := map[string]*layoutNode{}
	byPoint := map[string][]string{}
	for i := range layout.Nodes {
		n := &layout.Nodes[i]
		nodes[n.ID] = n
		for _, p := range n.pointIDs() {
			byPoint[p] = append(byPoint[p], n.ID)
		}
	}
	sameClass := func(o *cimObject, nodeID string) bool {
		n := nodes[nodeID]
		return n != nil && n.Data.CIMClass == "cim:"+o.Class
	}

	match := map[string]string{}
	used := map[string]bool{}
	assign := func(o *cimObject, nodeID string) {
		match[o.ID] = nodeID
		used[nodeID] = true
	}

	for _, o := range equipment {
		if len(o.PointIDs) == 0 {
			continue
		}
		var cands map[string]bool
		for _, p := range o.PointIDs {
			s := map[string]bool{}
			for _, id := range byPoint[p] {
				if (cands == nil || cands[id]) && sameClass(o, id) {
					s[id] = true
				}
			}
			cands = s
		}
		if id, ok := single(cands); ok && !used[id] {
			assign(o, id)
		}
	}

	for _, o := range equipment {
		if _, done := match[o.ID]; done || o.Class != "BusbarSection" {
			continue
		}
		bay := m.ByID[o.Ref["Equipment.EquipmentContainer"]]
		if bay == nil {
			continue
		}
		for _, name := range []string{bay.name(), bay.ID} {
			id, ok := strings.CutPrefix(name, "BAY_")
			if ok && sameClass(o, id) && !used[id] {
				assign(o, id)
				break
			}
		}
	}

	// CIM neighbors: equipment sharing a ConnectivityNode.
	cimNeighbors := map[string][]string{}
	for _, ts := range cnTerminals {
		for _, a := range ts {
			for _, b := range ts {
				if a.Equipment != b.Equipment {
					cimNeighbors[a.Equipment] = append(cimNeighbors[a.Equipment], b.Equipment)
				}
			}
		}
	}
	layoutNeighbors := map[string]map[string]bool{}
	link := func(a, b string) {
		if layoutNeighbors[a] == nil {
			layoutNeighbors[a] = map[string]bool{}
		}
		layoutNeighbors[a][b] = true
	}
	for _, e := range layout.Edges {
		link(e.Source, e.Target)
		link(e.Target, e.Source)
	}

	for changed := true; changed; {
		changed = false
		for _, o := range equipment {
			if _, done := match[o.ID]; done {
				continue
			}
			var cands map[string]bool
			for _, nb := range cimNeighbors[o.ID] {
				nbNode, ok := match[nb]
				if !ok {
					continue
				}
				s := map[string]bool{}
				for id := range layoutNeighbors[nbNode] {
					if (cands == nil || cands[id]) && !used[id] && sameClass(o, id) {
						s[id] = true
					}
				}
				cands = s
			}
			if id, ok := single(cands); ok {
				assign(o, id)
				changed = true
			}
		}
	}
	return match
}

func single(s map[string]bool) (string, bool) {
	if len(s) != 1 {
		return "", false
	}
	for k := range s {
		return k, true
	}
	return "", false
}
