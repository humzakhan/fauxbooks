package gen_test

import (
	"path/filepath"
	"testing"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

func TestLoadTemplatesConfig(t *testing.T) {
	t.Parallel()

	validPath := filepath.Join("testdata", "yaml", "valid-templates.yaml")

	t.Run("valid yaml returns populated TemplateDefinition slice", func(t *testing.T) {
		t.Parallel()
		defs, err := gen.LoadTemplatesConfig(validPath)
		if err != nil {
			t.Fatalf("LoadTemplatesConfig(%q): unexpected error: %v", validPath, err)
		}
		if len(defs) != 2 {
			t.Fatalf("len(defs) = %d; want 2", len(defs))
		}
		if defs[0].EventType != "sale_invoice" {
			t.Errorf("defs[0].EventType = %q; want %q", defs[0].EventType, "sale_invoice")
		}
		if defs[0].DebitAccount != "1100" {
			t.Errorf("defs[0].DebitAccount = %q; want %q", defs[0].DebitAccount, "1100")
		}
		if defs[0].CreditAccount != "4000" {
			t.Errorf("defs[0].CreditAccount = %q; want %q", defs[0].CreditAccount, "4000")
		}
		if defs[0].AmountField != "amount" {
			t.Errorf("defs[0].AmountField = %q; want %q", defs[0].AmountField, "amount")
		}
	})

	t.Run("file not found returns error not panic", func(t *testing.T) {
		t.Parallel()
		_, err := gen.LoadTemplatesConfig("/nonexistent/path/templates.yaml")
		if err == nil {
			t.Fatal("LoadTemplatesConfig: expected error for missing file, got nil")
		}
	})

	t.Run("unknown account code in debit_account returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
templates:
  - event_type: sale_invoice
    debit_account: "9999"
    credit_account: "4000"
    amount_field: amount
    description: "Bad debit account"
`)
		_, err := gen.LoadTemplatesConfig(f)
		if err == nil {
			t.Fatal("LoadTemplatesConfig: expected error for unknown debit_account, got nil")
		}
	})

	t.Run("unknown account code in credit_account returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
templates:
  - event_type: sale_invoice
    debit_account: "1100"
    credit_account: "9999"
    amount_field: amount
    description: "Bad credit account"
`)
		_, err := gen.LoadTemplatesConfig(f)
		if err == nil {
			t.Fatal("LoadTemplatesConfig: expected error for unknown credit_account, got nil")
		}
	})

	t.Run("missing credit_account returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
templates:
  - event_type: sale_invoice
    debit_account: "1100"
    amount_field: amount
    description: "Missing credit"
`)
		_, err := gen.LoadTemplatesConfig(f)
		if err == nil {
			t.Fatal("LoadTemplatesConfig: expected error for missing credit_account, got nil")
		}
	})

	t.Run("missing debit_account returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
templates:
  - event_type: sale_invoice
    credit_account: "4000"
    amount_field: amount
    description: "Missing debit"
`)
		_, err := gen.LoadTemplatesConfig(f)
		if err == nil {
			t.Fatal("LoadTemplatesConfig: expected error for missing debit_account, got nil")
		}
	})

	t.Run("invalid amount_field returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
templates:
  - event_type: sale_invoice
    debit_account: "1100"
    credit_account: "4000"
    amount_field: not_amount
    description: "Bad amount field"
`)
		_, err := gen.LoadTemplatesConfig(f)
		if err == nil {
			t.Fatal("LoadTemplatesConfig: expected error for invalid amount_field, got nil")
		}
	})

	t.Run("unknown event type returns error", func(t *testing.T) {
		t.Parallel()
		f := writeTempYAML(t, `
templates:
  - event_type: not_a_real_type
    debit_account: "1100"
    credit_account: "4000"
    amount_field: amount
    description: "Bad event type"
`)
		_, err := gen.LoadTemplatesConfig(f)
		if err == nil {
			t.Fatal("LoadTemplatesConfig: expected error for unknown event type, got nil")
		}
	})
}
