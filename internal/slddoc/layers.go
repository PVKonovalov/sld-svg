package slddoc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseLayers reads an SVG's <metadata> JSON payload
// ({"layers":[{"id":"..","label":".."}]}) and prepends the always-present
// BaseLayer, so every element has somewhere to point its layer reference
// even when the source SVG never wrote a data-layer attribute for it.
func parseLayers(metadataText string) ([]Layer, error) {
	layers := []Layer{{ID: BaseLayer, Name: "Base"}}

	text := strings.TrimSpace(metadataText)
	if text == "" {
		return layers, nil
	}

	var parsed struct {
		Layers []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"layers"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("slddoc: parsing metadata layers: %w", err)
	}
	for _, l := range parsed.Layers {
		layers = append(layers, Layer{ID: l.ID, Name: l.Label})
	}
	return layers, nil
}

func resolveLayer(dataLayer string) string {
	if dataLayer == "" {
		return BaseLayer
	}
	return dataLayer
}
