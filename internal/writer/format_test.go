package writer

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFormatDecimal(t *testing.T) {
	cases := []struct {
		name string
		in   decimal.Decimal
		want string
	}{
		{
			name: "zero",
			in:   decimal.Zero,
			want: "0.000000000",
		},
		{
			name: "neg-zero-float",
			// decimal.NewFromFloat(-0.0) is zero; IsZero() is true.
			in:   decimal.NewFromFloat(-0.0),
			want: "0.000000000",
		},
		{
			name: "neg-zero-neg",
			// decimal.Zero.Neg() produces -0 in shopspring; IsZero() is true.
			in:   decimal.Zero.Neg(),
			want: "0.000000000",
		},
		{
			name: "tiny",
			in:   decimal.NewFromFloat(0.000000001),
			want: "0.000000001",
		},
		{
			name: "typical",
			in:   decimal.NewFromFloat(9999.99),
			want: "9999.990000000",
		},
		{
			name: "negative",
			in:   decimal.NewFromFloat(-42.5),
			want: "-42.500000000",
		},
		{
			name: "integer",
			in:   decimal.NewFromInt(100),
			want: "100.000000000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatDecimal(tc.in)
			if got != tc.want {
				t.Errorf("FormatDecimal(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
