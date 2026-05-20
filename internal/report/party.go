package report

import (
	"fmt"
	"hash/fnv"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// deriveCustomerID deterministically maps an entryID to a customer ID from
// a pool of n customers. Uses FNV-1a 32-bit hash; no RNG.
func deriveCustomerID(entryID string, n int) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(entryID))
	index := int(h.Sum32()) % n
	return fmt.Sprintf("cust-%03d", index+1)
}

// deriveVendorID deterministically maps an entryID to a vendor ID from
// a pool of n vendors. Uses FNV-1a 32-bit hash; no RNG.
func deriveVendorID(entryID string, n int) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(entryID))
	index := int(h.Sum32()) % n
	return fmt.Sprintf("vend-%03d", index+1)
}

// customerName looks up the name of a customer by its derived ID.
// Returns the ID itself if not found (should not happen on valid data).
func customerName(id string, customers []gen.Customer) string {
	for _, c := range customers {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}

// vendorName looks up the name of a vendor by its derived ID.
// Returns the ID itself if not found (should not happen on valid data).
func vendorName(id string, vendors []gen.Vendor) string {
	for _, v := range vendors {
		if v.ID == id {
			return v.Name
		}
	}
	return id
}
