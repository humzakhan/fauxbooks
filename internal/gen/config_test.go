package gen

import (
	"errors"
	"testing"
)

func TestParseConfig_ErrBadPeriod(t *testing.T) {
	cases := []struct {
		name   string
		period string
	}{
		{"empty", ""},
		{"year-only", "2026"},
		{"invalid-month", "2026-13"},
		{"wrong-format", "01-2026"},
		{"full-date", "2026-01-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConfig(1, tc.period, "small", "monthly-close", "/tmp", "csv")
			if !errors.Is(err, ErrBadPeriod) {
				t.Errorf("period=%q: got %v, want ErrBadPeriod", tc.period, err)
			}
		})
	}
}

func TestParseConfig_ErrUnknownVolume(t *testing.T) {
	cases := []struct {
		name   string
		volume string
	}{
		{"empty", ""},
		{"typo", "smol"},
		{"uppercase", "Small"},
		{"extra", "xlarge"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConfig(1, "2026-01", tc.volume, "monthly-close", "/tmp", "csv")
			if !errors.Is(err, ErrUnknownVolume) {
				t.Errorf("volume=%q: got %v, want ErrUnknownVolume", tc.volume, err)
			}
		})
	}
}

func TestParseConfig_ErrUnknownBundle(t *testing.T) {
	cases := []struct {
		name   string
		bundle string
	}{
		{"empty", ""},
		{"typo", "montly-close"},
		{"future", "recon-verify"},
		{"unknown", "custom-bundle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConfig(1, "2026-01", "small", tc.bundle, "/tmp", "csv")
			if !errors.Is(err, ErrUnknownBundle) {
				t.Errorf("bundle=%q: got %v, want ErrUnknownBundle", tc.bundle, err)
			}
		})
	}
}

func TestParseConfig_ErrUnknownFormat(t *testing.T) {
	cases := []struct {
		name   string
		format string
	}{
		{"empty", ""},
		{"typo", "csvo"},
		{"uppercase", "CSV"},
		{"xml", "xml"},
		{"xlsx", "xlsx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConfig(1, "2026-01", "small", "monthly-close", "/tmp", tc.format)
			if !errors.Is(err, ErrUnknownFormat) {
				t.Errorf("format=%q: got %v, want ErrUnknownFormat", tc.format, err)
			}
		})
	}
}

func TestParseConfig_Valid(t *testing.T) {
	cfg, err := ParseConfig(42, "2026-01", "small", "monthly-close", "/tmp/out", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Seed != 42 {
		t.Errorf("Seed: got %d, want 42", cfg.Seed)
	}
	if cfg.Period.Year() != 2026 || cfg.Period.Month() != 1 {
		t.Errorf("Period: got %v, want 2026-01", cfg.Period)
	}
	if cfg.Period.Location().String() != "UTC" {
		t.Errorf("Period timezone: got %s, want UTC", cfg.Period.Location())
	}
	if cfg.Volume != "small" {
		t.Errorf("Volume: got %q, want %q", cfg.Volume, "small")
	}
	if cfg.Bundle != "monthly-close" {
		t.Errorf("Bundle: got %q, want %q", cfg.Bundle, "monthly-close")
	}
	if cfg.Format != "csv" {
		t.Errorf("Format: got %q, want %q", cfg.Format, "csv")
	}
}
