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
func Unverified() {} // want "Unverified documents an invariant"

// Descriptive returns the number of segments it was given. The documentation
// only describes; it asserts no property, so it is never reported.
func Descriptive() int { return 0 }

// Forged writes atomically, so no reader observes a partial value. A test
// SPELLS this symbol and does nothing whatever with it, which acquires no
// verification at all, so the claim is still unverified.
func Forged() {} // want "Forged documents an invariant"

// Mentioned writes atomically, so no reader observes a partial value. The test
// naming it refers to the symbol without calling it. That is the deliberate
// FLOOR of the exemption — the probe reads syntax and cannot judge what a test
// asserts — and it is cased here so the line is visible rather than assumed.
func Mentioned() {}

// Indirect writes atomically, so no reader observes a partial value. The test
// naming it reaches it only through a helper, so the test's own body mentions
// nothing. The probe reads ONE function body and does not follow calls, which
// is a documented scope limitation that errs toward reporting.
func Indirect() {} // want "Indirect documents an invariant"

// Unnamed writes atomically, so no reader observes a partial value. A test USES
// this symbol under a name that does not carry it. That is the other half of
// the exemption on its own, and half an exemption is not the exemption.
func Unnamed() {} // want "Unnamed documents an invariant"

// internalHelper always writes atomically. Unexported symbols are IN scope: a
// property claimed on an unexported helper is still a property nothing tests.
func internalHelper() {} // want "internalHelper documents an invariant"

// Cache is safe to copy: its state lives behind a reference field. A type
// declaration's claim counts, and no test names it.
type Cache struct{ store map[string]string } // want "Cache documents an invariant"

// Limit is always positive, a claim attached to a const declaration that no
// test names.
const Limit = 1 // want "Limit documents an invariant"

// Registry never returns nil, a claim attached to a var declaration that no
// test names.
var Registry = map[string]string{} // want "Registry documents an invariant"

// Plain has no doc-comment claim at all.
type Plain struct{}

// Sentinel errors, whose docs describe the CONDITION each reports rather than
// a property the code guarantees. The claim words here belong to the failure
// being described, so none of these is an invariant and none is reported.
const (
	// ErrCreateOutput is returned when the output directory cannot be created.
	ErrCreateOutput sentinelError = "failed to create output"
	// ErrExpire is returned when the expiration can never be computed.
	ErrExpire sentinelError = "failed to compute expiration"
)

// sentinelError is a string-typed error, the shape the fleet's errs.Const uses.
type sentinelError string

// Error satisfies the error interface.
func (e sentinelError) Error() string { return string(e) }
