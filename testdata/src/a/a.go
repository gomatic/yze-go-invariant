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

// Descriptive returns the number of segments it was given. This documentation
// only describes, and asserts no property at all.
func Descriptive() int { return 0 }

// Forged writes atomically, so no reader observes a partial value.
// Case: a test carries this symbol's name and does nothing whatever with it.
func Forged() {} // want "Forged documents an invariant"

// Mentioned writes atomically, so no reader observes a partial value.
// Case: the test carrying its name spells the symbol without calling it, which
// is the floor of the exemption.
func Mentioned() {}

// Indirect writes atomically, so no reader observes a partial value.
// Case: the test carrying its name reaches it only through a helper, so that
// test's own body spells nothing.
func Indirect() {} // want "Indirect documents an invariant"

// Copied is safe to copy: every field is a value.
// Case: the test carrying its name reaches it through a constructor, so that
// test's own body spells no such type. Same shape as Indirect, on the type
// path rather than the function one.
type Copied struct{ retries int } // want "Copied documents an invariant"

// NewCopied builds a Copied.
func NewCopied() Copied { return Copied{retries: 3} }

// Split writes atomically, so no reader observes a partial value.
// Case: one test carries the name and a DIFFERENT test spells the symbol. Two
// halves in two tests are not the exemption, and a corpus that pools a file's
// identifiers cannot tell this apart from the real thing.
func Split() {} // want "Split documents an invariant"

// Unnamed writes atomically, so no reader observes a partial value.
// Case: a test spells this symbol under a name that does not carry it.
func Unnamed() {} // want "Unnamed documents an invariant"

// blind always writes atomically.
// Case: the only test carrying its name sits in the EXTERNAL test package,
// which is unable to spell an unexported name at all.
func blind() {}

// unseen always writes atomically.
// Case: blind's in-scope sibling. An INTERNAL test carries its name and was
// able to spell it. Same claim, same shape, opposite verdict, and the only
// difference between them is which test package names it.
func unseen() {} // want "unseen documents an invariant"

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
