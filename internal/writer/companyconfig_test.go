package writer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/humzakhan/fauxbooks/internal/writer"
)

func TestDefaultCompanyConfig(t *testing.T) {
	cfg := writer.DefaultCompanyConfig()
	if cfg.CompanyName == "" {
		t.Error("DefaultCompanyConfig: CompanyName is empty")
	}
	if cfg.Address == "" {
		t.Error("DefaultCompanyConfig: Address is empty")
	}
	if cfg.Phone == "" {
		t.Error("DefaultCompanyConfig: Phone is empty")
	}
	if cfg.Email == "" {
		t.Error("DefaultCompanyConfig: Email is empty")
	}
}

func TestLoadCompanyConfigValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "company.yaml")
	content := `company_name: "Acme Corp"
address: "1 Main St, Springfield, IL 62701"
phone: "+1-800-555-1234"
email: "info@acme.example"
logo_path: "/tmp/logo.png"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := writer.LoadCompanyConfig(path)
	if err != nil {
		t.Fatalf("LoadCompanyConfig: %v", err)
	}
	if cfg.CompanyName != "Acme Corp" {
		t.Errorf("CompanyName: got %q, want %q", cfg.CompanyName, "Acme Corp")
	}
	if cfg.Address != "1 Main St, Springfield, IL 62701" {
		t.Errorf("Address: got %q", cfg.Address)
	}
	if cfg.Phone != "+1-800-555-1234" {
		t.Errorf("Phone: got %q", cfg.Phone)
	}
	if cfg.Email != "info@acme.example" {
		t.Errorf("Email: got %q", cfg.Email)
	}
	if cfg.LogoPath != "/tmp/logo.png" {
		t.Errorf("LogoPath: got %q", cfg.LogoPath)
	}
}

func TestLoadCompanyConfigMissingFile(t *testing.T) {
	_, err := writer.LoadCompanyConfig("/nonexistent/path/company.yaml")
	if err == nil {
		t.Error("LoadCompanyConfig: expected error for missing file, got nil")
	}
}

func TestLoadCompanyConfigMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	// Indentation error: tab character where spaces expected, and a raw tab
	// in a block scalar will cause a yaml parse failure.
	content := "company_name: [\nbad yaml here\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	_, err := writer.LoadCompanyConfig(path)
	if err == nil {
		t.Error("LoadCompanyConfig: expected error for malformed YAML, got nil")
	}
}

func TestLoadCompanyConfigPartialFields(t *testing.T) {
	// Missing fields should default to zero strings.
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	content := "company_name: \"Partial Corp\"\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := writer.LoadCompanyConfig(path)
	if err != nil {
		t.Fatalf("LoadCompanyConfig: %v", err)
	}
	if cfg.CompanyName != "Partial Corp" {
		t.Errorf("CompanyName: got %q", cfg.CompanyName)
	}
	if cfg.Phone != "" {
		t.Errorf("Phone should be empty, got %q", cfg.Phone)
	}
}
