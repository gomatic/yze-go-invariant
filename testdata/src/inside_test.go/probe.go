// Package inside sits in a DIRECTORY named "inside_test.go", which the go tool
// loads as an ordinary package: its files are compiled, linked and shipped, and
// none of them is a test file.
//
// The matcher decides on a PATH, so the "_test.go" the directory contributes is
// inside the path rather than at its end. A matcher widened from a suffix to a
// substring therefore spares every declaration in this package at once, and no
// fixture whose path holds "_test.go" only at the end can see it.
package inside

// Kept writes atomically, so no reader observes a partial value.
func Kept() {} // want "Kept documents an invariant"
