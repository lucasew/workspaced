package module

import (
	"encoding/json"
	"fmt"
)

// DecodeConfig converts the raw ModuleConfig (map[string]any coming from CUE)
// into your typed config struct.
//
// Usage inside a CoreModule:
//
//	cfg, err := module.DecodeConfig[placeConfig](req.ModuleConfig)
//	if err != nil { return nil, err }
//
// It does a JSON roundtrip because the raw config is untyped map data.
func DecodeConfig[C any](raw map[string]any) (C, error) {
	var c C
	if raw == nil {
		return c, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return c, fmt.Errorf("encode module config: %w", err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("decode module config: %w", err)
	}
	return c, nil
}
