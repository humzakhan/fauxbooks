package gen

import "fmt"

// customerCount is the fixed number of customer master records generated per run.
const customerCount = 10

// vendorCount is the fixed number of vendor master records generated per run.
const vendorCount = 10

// GenerateCustomers produces a deterministic slice of Customer records using the
// seeded RNG. The count is fixed so customer IDs are stable within a run.
// All fields derive from the RNG — no wall-clock, no global faker.
func GenerateCustomers(rng *RNG, count int) []Customer {
	customers := make([]Customer, count)
	for i := range customers {
		customers[i] = Customer{
			ID:      fmt.Sprintf("cust-%03d", i+1),
			Name:    rng.CompanyName(),
			Email:   rng.Email(),
			Phone:   rng.Phone(),
			Address: rng.Address(),
			City:    rng.City(),
			State:   rng.State(),
			Zip:     rng.Zip(),
		}
	}
	return customers
}

// GenerateVendors produces a deterministic slice of Vendor records using the
// seeded RNG. The count is fixed so vendor IDs are stable within a run.
// All fields derive from the RNG — no wall-clock, no global faker.
func GenerateVendors(rng *RNG, count int) []Vendor {
	vendors := make([]Vendor, count)
	for i := range vendors {
		vendors[i] = Vendor{
			ID:      fmt.Sprintf("vend-%03d", i+1),
			Name:    rng.CompanyName(),
			Email:   rng.Email(),
			Phone:   rng.Phone(),
			Address: rng.Address(),
			City:    rng.City(),
			State:   rng.State(),
			Zip:     rng.Zip(),
		}
	}
	return vendors
}
