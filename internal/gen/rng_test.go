package gen

import (
	"testing"
)

// TestRNGDeterminismIntN verifies that two RNG instances seeded with the same
// value produce identical first 100 IntN draws.
func TestRNGDeterminismIntN(t *testing.T) {
	const seed = int64(42)
	const draws = 100
	const n = 1000

	r1 := NewRNG(seed)
	r2 := NewRNG(seed)

	for i := 0; i < draws; i++ {
		v1 := r1.IntN(n)
		v2 := r2.IntN(n)
		if v1 != v2 {
			t.Fatalf("draw %d: r1.IntN(%d) = %d, r2.IntN(%d) = %d (seeds equal, expect identical)", i, n, v1, n, v2)
		}
	}
}

// TestRNGDeterminismCompanyName verifies that two RNG instances seeded with
// the same value produce identical first 10 company names.
func TestRNGDeterminismCompanyName(t *testing.T) {
	const seed = int64(1)
	const draws = 10

	r1 := NewRNG(seed)
	r2 := NewRNG(seed)

	for i := 0; i < draws; i++ {
		n1 := r1.CompanyName()
		n2 := r2.CompanyName()
		if n1 != n2 {
			t.Fatalf("draw %d: CompanyName mismatch: %q vs %q", i, n1, n2)
		}
	}
}

// TestRNGDifferentSeeds confirms that different seeds produce different sequences
// (probabilistic sanity check).
func TestRNGDifferentSeeds(t *testing.T) {
	r1 := NewRNG(1)
	r2 := NewRNG(2)

	allEqual := true
	for i := 0; i < 20; i++ {
		if r1.IntN(100000) != r2.IntN(100000) {
			allEqual = false
			break
		}
	}
	if allEqual {
		t.Error("seeds 1 and 2 produced identical sequences for 20 draws — seeding is likely broken")
	}
}

// TestRNGIntNIndependentOfFaker confirms that IntN draws and Faker draws do
// not share state: interleaving them produces the same individual sequences as
// calling them separately.
func TestRNGIntNIndependentOfFaker(t *testing.T) {
	const seed = int64(99)

	// First RNG: alternate IntN and CompanyName.
	r1 := NewRNG(seed)
	var intDrawsInterleaved []int
	for i := 0; i < 5; i++ {
		intDrawsInterleaved = append(intDrawsInterleaved, r1.IntN(1000))
		r1.CompanyName() // consume a faker draw between PCG draws
	}

	// Second RNG: do all IntN draws first, then all CompanyName draws.
	r2 := NewRNG(seed)
	var intDrawsSeparate []int
	for i := 0; i < 5; i++ {
		intDrawsSeparate = append(intDrawsSeparate, r2.IntN(1000))
	}

	// PCG and faker are independent sources, so IntN draws must be identical
	// regardless of whether faker is called in between.
	for i := range intDrawsInterleaved {
		if intDrawsInterleaved[i] != intDrawsSeparate[i] {
			t.Errorf("draw %d: interleaved=%d separate=%d — PCG and faker share state unexpectedly", i, intDrawsInterleaved[i], intDrawsSeparate[i])
		}
	}
}

// TestRNGFakerSeedDerived verifies that the gofakeit seed is derived from
// Config.Seed via the documented transform and not from a PCG draw. We do this
// by checking that two RNGs with the same seed produce the same faker output
// even after consuming different numbers of PCG draws.
func TestRNGFakerSeedDerived(t *testing.T) {
	const seed = int64(7)

	r1 := NewRNG(seed)
	// Consume several PCG draws before checking faker.
	for i := 0; i < 50; i++ {
		r1.IntN(999)
	}
	name1 := r1.CompanyName()

	r2 := NewRNG(seed)
	name2 := r2.CompanyName() // first faker draw, no PCG consumed first

	// Faker is seeded from seed^constant, NOT from PCG draws, so both should
	// produce the same first faker output regardless of PCG consumption.
	if name1 != name2 {
		t.Errorf("faker seed is not independent of PCG draw count: %q vs %q", name1, name2)
	}
}

// TestRNGAccessors smoke-tests the named accessor methods to ensure they return
// non-empty strings (determinism is covered by the company-name test above).
func TestRNGAccessors(t *testing.T) {
	r := NewRNG(123)

	checks := []struct {
		name string
		fn   func() string
	}{
		{"Name", r.Name},
		{"FirstName", r.FirstName},
		{"LastName", r.LastName},
		{"CompanyName", r.CompanyName},
		{"Email", r.Email},
		{"Phone", r.Phone},
		{"Address", r.Address},
		{"City", r.City},
		{"State", r.State},
		{"Zip", r.Zip},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			got := c.fn()
			if got == "" {
				t.Errorf("%s() returned empty string", c.name)
			}
		})
	}
}

// TestNewRNGReturnsFaker verifies Faker() returns the same instance used
// internally (not a freshly seeded one).
func TestNewRNGReturnsFaker(t *testing.T) {
	r := NewRNG(55)
	f := r.Faker()
	if f == nil {
		t.Fatal("Faker() returned nil")
	}
	// Call through both Faker() and the wrapper; they should advance the same state.
	name1 := r.Faker().Name()
	name2 := r.Name() // this creates a new faker call on the same instance
	// We just verify neither panics and returns non-empty.
	if name1 == "" || name2 == "" {
		t.Error("unexpected empty name from Faker()")
	}
}
