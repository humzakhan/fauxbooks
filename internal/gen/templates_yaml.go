package gen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TemplateDefinition describes a custom journal-entry template loaded from YAML.
// Each template maps a single event type to a two-sided (debit/credit) journal
// entry. Only event types registered in allEventTypes are supported; only
// account codes present in DefaultCoA() may be referenced.
//
// AmountField must be "amount" — the only metadata key supported in v0.2.
// This constraint is intentional: all events carry an Amount field, and custom
// templates that split amounts or reference other fields are deferred to v0.3.
type TemplateDefinition struct {
	// EventType is the event type this template handles (e.g. "sale_invoice").
	EventType string

	// DebitAccount is the account code to debit (must exist in DefaultCoA).
	DebitAccount string

	// CreditAccount is the account code to credit (must exist in DefaultCoA).
	CreditAccount string

	// AmountField is the event metadata key whose value provides the entry
	// amount. Must be "amount" in v0.2.
	AmountField string

	// Description is the human-readable description written to the journal entry.
	Description string
}

// templatesYAML is the on-disk YAML shape for a list of TemplateDefinition values.
type templatesYAML struct {
	Templates []templateEntryYAML `yaml:"templates"`
}

type templateEntryYAML struct {
	EventType     string `yaml:"event_type"`
	DebitAccount  string `yaml:"debit_account"`
	CreditAccount string `yaml:"credit_account"`
	AmountField   string `yaml:"amount_field"`
	Description   string `yaml:"description"`
}

// LoadTemplatesConfig reads a YAML file at path and returns a validated slice
// of TemplateDefinition values. The YAML schema is:
//
//	templates:
//	  - event_type: sale_invoice
//	    debit_account: "1100"
//	    credit_account: "4000"
//	    amount_field: amount
//	    description: "Sale invoice"
//
// Validation rules:
//   - debit_account and credit_account must both be set and exist in DefaultCoA().
//   - amount_field must be "amount" (the only supported value in v0.2).
//   - event_type must be a registered event type from allEventTypes.
//
// Returns a clear error prefixed with "LoadTemplatesConfig: " on any failure.
// Never panics.
func LoadTemplatesConfig(path string) ([]TemplateDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadTemplatesConfig: read %s: %w", path, err)
	}

	var raw templatesYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("LoadTemplatesConfig: parse %s: %w", path, err)
	}

	if len(raw.Templates) == 0 {
		return nil, fmt.Errorf("LoadTemplatesConfig: templates list must not be empty")
	}

	// Build the set of valid event type strings.
	validTypes := make(map[string]bool, len(allEventTypes))
	for _, et := range allEventTypes {
		validTypes[string(et)] = true
	}

	// Build the set of valid account codes from DefaultCoA.
	coaMap := IndexCoA(DefaultCoA())

	out := make([]TemplateDefinition, 0, len(raw.Templates))
	for i, entry := range raw.Templates {
		if !validTypes[entry.EventType] {
			return nil, fmt.Errorf("LoadTemplatesConfig: templates[%d]: unknown event type %q", i, entry.EventType)
		}
		if entry.DebitAccount == "" {
			return nil, fmt.Errorf("LoadTemplatesConfig: templates[%d]: debit_account must be set", i)
		}
		if entry.CreditAccount == "" {
			return nil, fmt.Errorf("LoadTemplatesConfig: templates[%d]: credit_account must be set", i)
		}
		if _, ok := coaMap[entry.DebitAccount]; !ok {
			return nil, fmt.Errorf("LoadTemplatesConfig: templates[%d]: debit_account %q not found in DefaultCoA", i, entry.DebitAccount)
		}
		if _, ok := coaMap[entry.CreditAccount]; !ok {
			return nil, fmt.Errorf("LoadTemplatesConfig: templates[%d]: credit_account %q not found in DefaultCoA", i, entry.CreditAccount)
		}
		if entry.AmountField != "amount" {
			return nil, fmt.Errorf("LoadTemplatesConfig: templates[%d]: amount_field must be \"amount\" (only supported value in v0.2), got %q", i, entry.AmountField)
		}

		out = append(out, TemplateDefinition{
			EventType:     entry.EventType,
			DebitAccount:  entry.DebitAccount,
			CreditAccount: entry.CreditAccount,
			AmountField:   entry.AmountField,
			Description:   entry.Description,
		})
	}

	return out, nil
}
