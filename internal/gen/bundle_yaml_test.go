package gen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

func TestLoadBundleConfig(t *testing.T) {
	t.Parallel()

	validPath := filepath.Join("testdata", "yaml", "valid-bundle.yaml")

	t.Run("valid yaml returns populated BundleDefinition", func(t *testing.T) {
		t.Parallel()
		bd, err := gen.LoadBundleConfig(validPath)
		if err != nil {
			t.Fatalf("LoadBundleConfig(%q): unexpected error: %v", validPath, err)
		}
		if bd.Name != "tiny-test" {
			t.Errorf("Name = %q; want %q", bd.Name, "tiny-test")
		}
		if bd.StressTest {
			t.Error("StressTest = true; want false")
		}
		if bd.EventMix["sale_invoice"] != 5 {
			t.Errorf("EventMix[sale_invoice] = %d; want 5", bd.EventMix["sale_invoice"])
		}
		if bd.EventMix["vendor_bill"] != 5 {
			t.Errorf("EventMix[vendor_bill] = %d; want 5", bd.EventMix["vendor_bill"])
		}
		if bd.VolumeScale["small"] != 10 {
			t.Errorf("VolumeScale[small] = %d; want 10", bd.VolumeScale["small"])
		}
		if bd.VolumeScale["medium"] != 50 {
			t.Errorf("VolumeScale[medium] = %d; want 50", bd.VolumeScale["medium"])
		}
		if bd.VolumeScale["large"] != 100 {
			t.Errorf("VolumeScale[large] = %d; want 100", bd.VolumeScale["large"])
		}
	})

	t.Run("file not found returns error not panic", func(t *testing.T) {
		t.Parallel()
		_, err := gen.LoadBundleConfig("/nonexistent/path/bundle.yaml")
		if err == nil {
			t.Fatal("LoadBundleConfig: expected error for missing file, got nil")
		}
	})

	t.Run("empty name returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
stress_test: false
event_mix:
  sale_invoice: 5
volume_scale:
  small: 10
  medium: 50
  large: 100
`)
		_, err := gen.LoadBundleConfig(f)
		if err == nil {
			t.Fatal("LoadBundleConfig: expected error for empty name, got nil")
		}
	})

	t.Run("negative weight returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
name: bad-weights
stress_test: false
event_mix:
  sale_invoice: -1
volume_scale:
  small: 10
  medium: 50
  large: 100
`)
		_, err := gen.LoadBundleConfig(f)
		if err == nil {
			t.Fatal("LoadBundleConfig: expected error for negative weight, got nil")
		}
	})

	t.Run("zero weight returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
name: zero-weight
stress_test: false
event_mix:
  sale_invoice: 0
volume_scale:
  small: 10
  medium: 50
  large: 100
`)
		_, err := gen.LoadBundleConfig(f)
		if err == nil {
			t.Fatal("LoadBundleConfig: expected error for zero weight, got nil")
		}
	})

	t.Run("unknown event type returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
name: bad-type
stress_test: false
event_mix:
  not_a_real_type: 5
volume_scale:
  small: 10
  medium: 50
  large: 100
`)
		_, err := gen.LoadBundleConfig(f)
		if err == nil {
			t.Fatal("LoadBundleConfig: expected error for unknown event type, got nil")
		}
	})

	t.Run("missing volume tier returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
name: missing-tier
stress_test: false
event_mix:
  sale_invoice: 5
volume_scale:
  small: 10
  medium: 50
`)
		_, err := gen.LoadBundleConfig(f)
		if err == nil {
			t.Fatal("LoadBundleConfig: expected error for missing volume tier, got nil")
		}
	})

	t.Run("negative volume count returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
name: neg-vol
stress_test: false
event_mix:
  sale_invoice: 5
volume_scale:
  small: -1
  medium: 50
  large: 100
`)
		_, err := gen.LoadBundleConfig(f)
		if err == nil {
			t.Fatal("LoadBundleConfig: expected error for negative volume count, got nil")
		}
	})
}

// writeTempYAML writes content to a temp file and returns its path.
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	return f.Name()
}
