package gen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// bundleYAML is the on-disk YAML shape for a BundleDefinition.
// Field names use snake_case to match the documented YAML schema.
type bundleYAML struct {
	Name        string         `yaml:"name"`
	StressTest  bool           `yaml:"stress_test"`
	EventMix    map[string]int `yaml:"event_mix"`
	VolumeScale map[string]int `yaml:"volume_scale"`
}

// LoadBundleConfig reads a YAML file at path and returns a validated
// BundleDefinition. The YAML schema is:
//
//	name: my-bundle
//	stress_test: false
//	event_mix:
//	  sale_invoice: 50
//	  vendor_bill:  30
//	volume_scale:
//	  small:  100
//	  medium: 1000
//	  large:  10000
//
// Validation rules:
//   - name must be non-empty.
//   - event_mix: all values must be positive integers; all keys must be valid
//     registered event types (as listed in allEventTypes in events.go).
//   - volume_scale must include all three tiers ("small", "medium", "large");
//     all values must be positive.
//
// Returns a clear error prefixed with "LoadBundleConfig: " on any failure.
// Never panics.
func LoadBundleConfig(path string) (BundleDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: read %s: %w", path, err)
	}

	var raw bundleYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: parse %s: %w", path, err)
	}

	if raw.Name == "" {
		return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: name must be non-empty")
	}

	if len(raw.EventMix) == 0 {
		return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: event_mix must have at least one entry")
	}

	// Build the set of valid event type strings from allEventTypes.
	validTypes := make(map[string]bool, len(allEventTypes))
	for _, et := range allEventTypes {
		validTypes[string(et)] = true
	}

	mix := make(EventMix, len(raw.EventMix))
	for k, v := range raw.EventMix {
		if !validTypes[k] {
			return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: event_mix: unknown event type %q", k)
		}
		if v <= 0 {
			return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: event_mix: weight for %q must be positive, got %d", k, v)
		}
		mix[EventType(k)] = v
	}

	// Validate volume_scale: must include all three tiers with positive values.
	requiredTiers := []string{"small", "medium", "large"}
	for _, tier := range requiredTiers {
		v, ok := raw.VolumeScale[tier]
		if !ok {
			return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: volume_scale: missing required tier %q", tier)
		}
		if v <= 0 {
			return BundleDefinition{}, fmt.Errorf("LoadBundleConfig: volume_scale: count for tier %q must be positive, got %d", tier, v)
		}
	}

	return BundleDefinition{
		Name:        raw.Name,
		EventMix:    mix,
		VolumeScale: raw.VolumeScale,
		StressTest:  raw.StressTest,
	}, nil
}
