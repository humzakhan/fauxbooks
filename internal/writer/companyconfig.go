// Package writer — company configuration for PDF rendering.
// CompanyConfig holds per-tenant branding fields used in PDF headers.
// No file I/O, no time.Now(), no RNG in this package. LoadCompanyConfig is
// the only function that does I/O; DefaultCompanyConfig returns a hard-coded
// sentinel value suitable for CI golden fixtures.
package writer

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CompanyConfig holds per-tenant branding information used in PDF headers.
// All fields are optional: missing fields fall back to the zero string.
// LogoPath is reserved for v0.3 logo embedding; in v0.2 it is read from YAML
// but not rendered into the PDF.
type CompanyConfig struct {
	// CompanyName is printed as the first line of the PDF header.
	CompanyName string `yaml:"company_name"`
	// Address is printed below the company name in the PDF header.
	Address string `yaml:"address"`
	// Phone is printed on the same line as Email in the PDF header.
	Phone string `yaml:"phone"`
	// Email is printed on the same line as Phone in the PDF header.
	Email string `yaml:"email"`
	// LogoPath is a filesystem path to a PNG/JPEG logo image.
	// Logo rendering is deferred to v0.3; this field is parsed but not used.
	LogoPath string `yaml:"logo_path"`
}

// DefaultCompanyConfig returns a hard-coded CompanyConfig suitable for use in
// CI golden fixtures when no --company-config flag is provided.
// It never calls time.Now(), the RNG, or os.Getenv.
func DefaultCompanyConfig() CompanyConfig {
	return CompanyConfig{
		CompanyName: "Fauxbooks Demo Company",
		Address:     "100 Test Street, Sandbox, CA 94000",
		Phone:       "+1-555-0100",
		Email:       "demo@fauxbooks.example",
		LogoPath:    "",
	}
}

// LoadCompanyConfig reads and parses the YAML file at path and returns a
// populated CompanyConfig. It returns an error (not a panic) if the file is
// missing, unreadable, or contains malformed YAML. Unknown YAML keys are
// ignored to allow future fields to be added without breaking older binaries.
func LoadCompanyConfig(path string) (CompanyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CompanyConfig{}, fmt.Errorf("LoadCompanyConfig: read %s: %w", path, err)
	}
	var cfg CompanyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return CompanyConfig{}, fmt.Errorf("LoadCompanyConfig: parse %s: %w", path, err)
	}
	return cfg, nil
}
