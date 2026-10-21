package cimimport

import (
	"encoding/json"
	"fmt"
	"io"
)

// layoutDoc is the subset of a React Flow SLD export Import reads: a flat
// node list (one placed symbol each, Position its box's top-left corner) and
// the edges drawn between them, each naming which handle of each box it
// attaches to ("t"/"b"/"l"/"r", or "t1".."tN" along a busbar).
type layoutDoc struct {
	Nodes []layoutNode `json:"nodes"`
	Edges []layoutEdge `json:"edges"`
}

type layoutPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type layoutNode struct {
	ID               string       `json:"id"`
	Type             string       `json:"type"`
	Position         layoutPoint  `json:"position"`
	PositionAbsolute *layoutPoint `json:"positionAbsolute"`
	Style            struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"style"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Data   struct {
		CIMClass string  `json:"cimClass"`
		Rotation int     `json:"rotation"`
		Text     string  `json:"text"`
		FontSize float64 `json:"fontSize"`
		Bold     bool    `json:"bold"`
		Color    string  `json:"color"`
		Points   []struct {
			PointID string `json:"pointId"`
		} `json:"points"`
		LiveValues map[string]json.RawMessage `json:"liveValues"`
	} `json:"data"`
}

type layoutEdge struct {
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle"`
	Target       string `json:"target"`
	TargetHandle string `json:"targetHandle"`
}

func parseLayout(r io.Reader) (*layoutDoc, error) {
	var d layoutDoc
	if err := json.NewDecoder(r).Decode(&d); err != nil {
		return nil, fmt.Errorf("cimimport: parsing layout JSON: %w", err)
	}
	return &d, nil
}

// box is a layout node's absolute bounding box.
type box struct{ X, Y, W, H float64 }

func (n *layoutNode) box() box {
	p := n.Position
	if n.PositionAbsolute != nil {
		p = *n.PositionAbsolute
	}
	w, h := n.Style.Width, n.Style.Height
	if w == 0 {
		w = n.Width
	}
	if h == 0 {
		h = n.Height
	}
	if w == 0 {
		w = 20
	}
	if h == 0 {
		h = 20
	}
	return box{X: p.X, Y: p.Y, W: w, H: h}
}

func (b box) center() point { return point{b.X + b.W/2, b.Y + b.H/2} }

// handlePoint is where a handle sits on the box's own (unrotated) outline,
// as React Flow draws it.
func (b box) handlePoint(handle string) point {
	switch handle {
	case "t":
		return point{b.X + b.W/2, b.Y}
	case "b":
		return point{b.X + b.W/2, b.Y + b.H}
	case "l":
		return point{b.X, b.Y + b.H/2}
	case "r":
		return point{b.X + b.W, b.Y + b.H/2}
	}
	return b.center()
}

// pointIDs is every SCADA point id the node is bound to.
func (n *layoutNode) pointIDs() []string {
	var ids []string
	for _, p := range n.Data.Points {
		if p.PointID != "" {
			ids = append(ids, p.PointID)
		}
	}
	for k := range n.Data.LiveValues {
		ids = append(ids, k)
	}
	return ids
}
