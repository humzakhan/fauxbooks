package invariant

import "github.com/humzakhan/fauxbooks/internal/gen"

// InvariantError is a type alias for gen.InvariantError so callers can use
// either invariant.InvariantError or gen.InvariantError interchangeably, and
// errors.As works with a pointer to either form.
//
// Design choice: InvariantError was placed in internal/gen/types.go by PR-02
// because Dataset and its sibling types live there. Moving it to this package
// would require gen to import invariant (a cycle) or a third package. The alias
// is the zero-disruption solution: one line here, no import cycle, full errors.As
// compatibility.
type InvariantError = gen.InvariantError
