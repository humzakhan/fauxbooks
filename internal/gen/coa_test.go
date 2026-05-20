package gen

import (
	"reflect"
	"testing"
)

// TestDefaultCoACount asserts the exact account count.
func TestDefaultCoACount(t *testing.T) {
	accounts := DefaultCoA()
	if len(accounts) != DefaultCoACount {
		t.Errorf("DefaultCoA() len = %d, want %d", len(accounts), DefaultCoACount)
	}
}

// TestDefaultCoANoDuplicates verifies that no two accounts share the same code.
func TestDefaultCoANoDuplicates(t *testing.T) {
	accounts := DefaultCoA()
	seen := make(map[string]int, len(accounts))
	for i, a := range accounts {
		if prev, ok := seen[a.Code]; ok {
			t.Errorf("duplicate code %q at indices %d and %d", a.Code, prev, i)
		}
		seen[a.Code] = i
	}
}

// TestDefaultCoANormalBalance verifies every NormalBalance value is "debit" or "credit".
func TestDefaultCoANormalBalance(t *testing.T) {
	for _, a := range DefaultCoA() {
		if a.NormalBalance != "debit" && a.NormalBalance != "credit" {
			t.Errorf("account %q has invalid NormalBalance %q", a.Code, a.NormalBalance)
		}
	}
}

// TestDefaultCoADeterministic verifies two calls return deeply-equal slices in
// the same order, confirming the function is deterministic across calls.
func TestDefaultCoADeterministic(t *testing.T) {
	a := DefaultCoA()
	b := DefaultCoA()
	if !reflect.DeepEqual(a, b) {
		t.Error("DefaultCoA() returned different slices on consecutive calls")
	}
}

// TestIndexCoANoDuplicates feeds DefaultCoA into IndexCoA and checks no codes
// are lost (length match is a proxy for no silent overwrites).
func TestIndexCoANoDuplicates(t *testing.T) {
	accounts := DefaultCoA()
	index := IndexCoA(accounts)

	if len(index) != len(accounts) {
		t.Errorf("IndexCoA len = %d, want %d (possible duplicates)", len(index), len(accounts))
	}

	// Verify every account is reachable by code.
	for _, a := range accounts {
		if got, ok := index[a.Code]; !ok {
			t.Errorf("code %q missing from index", a.Code)
		} else if got.Code != a.Code {
			t.Errorf("index[%q].Code = %q, want %q", a.Code, got.Code, a.Code)
		}
	}
}

// TestCashAccountCodesInDefaultCoA confirms that every code in cashAccountCodes
// exists in DefaultCoA and that at least one cash code is present.
func TestCashAccountCodesInDefaultCoA(t *testing.T) {
	index := IndexCoA(DefaultCoA())

	if len(cashAccountCodes) == 0 {
		t.Fatal("cashAccountCodes is empty; at least one cash account is required")
	}

	for code := range cashAccountCodes {
		if _, ok := index[code]; !ok {
			t.Errorf("cash account code %q is in cashAccountCodes but not in DefaultCoA()", code)
		}
	}
}

// TestDefaultCoAAccountTypes verifies every AccountType value is one of the
// five valid types.
func TestDefaultCoAAccountTypes(t *testing.T) {
	validTypes := map[AccountType]bool{
		AccountTypeAsset:     true,
		AccountTypeLiability: true,
		AccountTypeEquity:    true,
		AccountTypeRevenue:   true,
		AccountTypeExpense:   true,
	}
	for _, a := range DefaultCoA() {
		if !validTypes[a.Type] {
			t.Errorf("account %q has invalid Type %q", a.Code, a.Type)
		}
	}
}

// TestDefaultCoAMutation confirms that mutating a returned slice does not affect
// subsequent calls (DefaultCoA returns a copy, not the internal slice).
func TestDefaultCoAMutation(t *testing.T) {
	a := DefaultCoA()
	originalCode := a[0].Code
	a[0].Code = "MUTATED"

	b := DefaultCoA()
	if b[0].Code != originalCode {
		t.Errorf("DefaultCoA() internal slice was mutated: got %q, want %q", b[0].Code, originalCode)
	}
}
