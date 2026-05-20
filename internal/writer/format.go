// Package writer implements streaming CSV and JSONL output writers for fauxbooks.
// All monetary values are serialized as decimal text with exactly 9 fractional
// digits (e.g. "100.000000000"). No float64, no scientific notation.
package writer

import "github.com/shopspring/decimal"

// FormatDecimal serializes d as plain decimal text with exactly 9 fractional
// digits: no thousands separator, no exponent, no trailing sign for zero.
// Neg-zero guard: any value for which d.IsZero() is true returns "0.000000000",
// never "-0.000000000".
func FormatDecimal(d decimal.Decimal) string {
	if d.IsZero() {
		return "0.000000000"
	}
	return d.StringFixed(9)
}
