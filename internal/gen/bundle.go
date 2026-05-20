package gen

import "fmt"

// BundleDefinition describes a named event mix that drives a generation run.
// The built-in bundles are returned by MonthlyClose and ReconStressTest;
// YAML-loaded bundles are returned by LoadBundleConfig in bundle_yaml.go.
//
// All fields are value types. BundleDefinition is immutable once constructed.
type BundleDefinition struct {
	// Name is the canonical bundle identifier (e.g. "monthly-close").
	Name string

	// EventMix is the event type distribution for this bundle.
	// Weights are positive integers; types with weight 0 are excluded.
	// The map key is an EventType (string) → integer weight.
	EventMix EventMix

	// VolumeScale maps volume tier names ("small", "medium", "large") to
	// row/event count targets. Used by YAML-loaded bundles to override the
	// built-in volumeEventCount values. Nil or empty means use built-in counts.
	VolumeScale map[string]int

	// StressTest indicates whether the bundle enables the second injection pass
	// (recon-stress-test only). False for all standard bundles.
	StressTest bool
}

// MonthlyClose returns the canonical monthly-close bundle definition.
// Weights reflect a typical month-end close: invoices and payments dominate,
// with smaller shares for bills, expenses, payroll, and depreciation.
func MonthlyClose() BundleDefinition {
	return BundleDefinition{
		Name: "monthly-close",
		EventMix: EventMix{
			EventTypeSaleInvoice:       10,
			EventTypeCustomerPayment:   10,
			EventTypeVendorBill:        10,
			EventTypeBillPayment:       10,
			EventTypeExpenseReport:     5,
			EventTypePayroll:           5,
			EventTypeDepreciation:      3,
			EventTypeInventoryPurchase: 7,
			EventTypeInventoryCOGS:     7,
			EventTypeInterestIncome:    3,
		},
		StressTest: false,
	}
}

// ReconStressTest returns the recon-stress-test bundle definition.
// It uses the same event mix as MonthlyClose and enables the stress injection
// pass (StressTest: true), which calls InjectStress after projection and writes
// truth.json alongside all standard documents.
func ReconStressTest() BundleDefinition {
	return BundleDefinition{
		Name: "recon-stress-test",
		EventMix: EventMix{
			EventTypeSaleInvoice:       10,
			EventTypeCustomerPayment:   10,
			EventTypeVendorBill:        10,
			EventTypeBillPayment:       10,
			EventTypeExpenseReport:     5,
			EventTypePayroll:           5,
			EventTypeDepreciation:      3,
			EventTypeInventoryPurchase: 7,
			EventTypeInventoryCOGS:     7,
			EventTypeInterestIncome:    3,
		},
		StressTest: true,
	}
}

// bundleByName returns the BundleDefinition for the given name, or ErrUnknownBundle if
// the name is not recognised. An empty name resolves to "monthly-close".
func bundleByName(name string) (BundleDefinition, error) {
	switch name {
	case "monthly-close", "":
		return MonthlyClose(), nil
	case "recon-stress-test":
		return ReconStressTest(), nil
	default:
		return BundleDefinition{}, fmt.Errorf("%w: %s", ErrUnknownBundle, name)
	}
}
