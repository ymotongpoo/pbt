// Package pbt is a property-based testing library for Go.
//
// A property is a function that draws random inputs from generators and
// checks that some statement holds for them. [Check] runs a property many
// times with inputs of growing size. When the property fails, pbt shrinks
// the failing input to a minimal one, saves it under testdata/pbt so that
// go test replays it from then on, and reports every value the property
// drew.
//
// Generators are deterministic functions of a sequence of choices, and
// shrinking rewrites that sequence rather than the generated values. That
// is why shrinking composes through [Map], [Bind], [Filter] and
// user-written generators without any shrinking code.
//
// The package is under development; see DESIGN.md in the repository.
package pbt
