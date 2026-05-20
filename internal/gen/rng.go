package gen

import (
	"math/rand/v2"

	"github.com/brianvoe/gofakeit/v7"
)

// fakerSeedMix is XOR'd with Config.Seed to derive the gofakeit seed.
// Using a fixed constant ensures the gofakeit seed is a stable, documentable
// function of Config.Seed and is never a subsequent PCG draw (which would
// shift all future RNG draws and break determinism).
//
// Value chosen as a large prime that is unlikely to collide with common seeds.
const fakerSeedMix = int64(0x517cc1b727220a95)

// RNG is a deterministic random source for the generation path.
// It wraps a math/rand/v2 PCG seeded exclusively from Config.Seed and a
// gofakeit.Faker seeded from a fixed transform of Config.Seed.
// No global random sources are used anywhere in internal/.
type RNG struct {
	pcg   *rand.Rand
	faker *gofakeit.Faker
}

// NewRNG constructs an RNG seeded from the provided seed value.
//
// PCG seeding: rand/v2 PCG takes two uint64 state words. We derive them
// deterministically from seed as:
//
//	state  = uint64(seed)
//	stream = uint64(seed>>32) ^ 0xdeadbeefcafe1234
//
// gofakeit seeding: the Faker is seeded with (seed ^ fakerSeedMix), which
// is a stable function of seed and never consumes a PCG draw.
func NewRNG(seed int64) *RNG {
	state := uint64(seed)
	stream := uint64(seed>>32) ^ 0xdeadbeefcafe1234

	src := rand.NewPCG(state, stream)
	pcg := rand.New(src)

	fakerSeed := uint64(seed ^ fakerSeedMix)
	faker := gofakeit.New(fakerSeed)

	return &RNG{pcg: pcg, faker: faker}
}

// IntN returns a non-negative pseudo-random int in [0, n).
// Panics if n <= 0, matching the behaviour of math/rand/v2.
func (r *RNG) IntN(n int) int {
	return r.pcg.IntN(n)
}

// Int64N returns a non-negative pseudo-random int64 in [0, n).
func (r *RNG) Int64N(n int64) int64 {
	return r.pcg.Int64N(n)
}

// Name returns a deterministic fake full name.
func (r *RNG) Name() string {
	return r.faker.Name()
}

// FirstName returns a deterministic fake first name.
func (r *RNG) FirstName() string {
	return r.faker.FirstName()
}

// LastName returns a deterministic fake last name.
func (r *RNG) LastName() string {
	return r.faker.LastName()
}

// CompanyName returns a deterministic fake company name.
func (r *RNG) CompanyName() string {
	return r.faker.Company()
}

// Email returns a deterministic fake email address.
func (r *RNG) Email() string {
	return r.faker.Email()
}

// Phone returns a deterministic fake phone number.
func (r *RNG) Phone() string {
	return r.faker.Phone()
}

// Address returns a deterministic fake street address.
func (r *RNG) Address() string {
	return r.faker.Street()
}

// City returns a deterministic fake city name.
func (r *RNG) City() string {
	return r.faker.City()
}

// State returns a deterministic fake US state abbreviation.
func (r *RNG) State() string {
	return r.faker.StateAbr()
}

// Zip returns a deterministic fake US ZIP code.
func (r *RNG) Zip() string {
	return r.faker.Zip()
}

// Faker returns the underlying gofakeit.Faker so callers in later PRs can
// access faker methods not yet wrapped here without adding new pass-throughs.
func (r *RNG) Faker() *gofakeit.Faker {
	return r.faker
}
