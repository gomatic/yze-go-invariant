// Package a pins the invariant contract: an exported symbol whose doc comment
// ASSERTS a property is reported unless some test function names that symbol.
package a

// The import declaration is a GenDecl whose specs are imports, exercising the
// path where a declaration introduces no documented symbol.
import "strings"

// Joined keeps the import used.
func Joined(parts []string) string { return strings.Join(parts, ",") }

// Verified writes atomically, so no reader observes a partial value. A test
// names this symbol, so the claim is treated as verified.
func Verified() {}

// Unverified writes atomically, so no reader observes a partial value. No test
// names this symbol, so the claim is unverified.
func Unverified() {}

// Descriptive returns the number of segments it was given. The documentation
// only describes; it asserts no property, so it is never reported.
func Descriptive() int { return 0 }

// internalHelper always writes atomically. Unexported symbols are IN scope: a
// property claimed on an unexported helper is still a property nothing tests.
func internalHelper() {}

// Cache is safe to copy: its state lives behind a reference field. A type
// declaration's claim counts, and no test names it.
type Cache struct{ store map[string]string }

// Limit is always positive, a claim attached to a const declaration that no
// test names.
const Limit = 1

// Registry never returns nil, a claim attached to a var declaration that no
// test names.
var Registry = map[string]string{}

// Plain has no doc-comment claim at all.
type Plain struct{}
