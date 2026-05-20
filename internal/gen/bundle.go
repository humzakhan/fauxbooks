package gen

import "fmt"

// Bundle describes a named event mix that drives a generation run.
// Each bundle is hardcoded in this file; no external configuration is read.
type Bundle struct {
	// Name is the canonical bundle identifier (e.g. "monthly-close").
	Name string

	// Mix is the event type distribution for this bundle.
	// Weights are positive integers; types with weight 0 are excluded.
	Mix EventMix

	// StressTest indicates whether the bundle enables the second injection pass
	// (recon-stress-test only). False for all v0.1 standard bundles.
	StressTest bool
}

// MonthlyClose returns the canonical monthly-close bundle definition.
// Weights reflect a typical month-end close: invoices and payments dominate,
// with smaller shares for bills, expenses, payroll, and depreciation.
func MonthlyClose() Bundle {
	return Bundle{
		Name: "monthly-close",
		Mix: EventMix{
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
func ReconStressTest() Bundle {
	return Bundle{
		Name: "recon-stress-test",
		Mix: EventMix{
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

// bundleByName returns the Bundle for the given name, or ErrUnknownBundle if
// the name is not recognised. An empty name resolves to "monthly-close".
func bundleByName(name string) (Bundle, error) {
	switch name {
	case "monthly-close", "":
		return MonthlyClose(), nil
	case "recon-stress-test":
		return ReconStressTest(), nil
	default:
		return Bundle{}, fmt.Errorf("%w: %s", ErrUnknownBundle, name)
	}
}
