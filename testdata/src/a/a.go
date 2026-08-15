// Package a pins the invariant contract: a symbol whose doc comment ASSERTS a
// property is reported unless some ONE test function both names that symbol and
// reaches it.
package a

// The import declaration is a GenDecl whose specs are imports, exercising the
// path where a declaration introduces no documented symbol.
import "strings"

// Joined keeps the import used.
func Joined(parts []string) string { return strings.Join(parts, ",") }

// Verified writes atomically, so no reader observes a partial value. The test
// carrying its name calls it in its own body.
func Verified() {}

// Unverified writes atomically, so no reader observes a partial value. No test
// names this symbol, so the claim is unverified.
// The whole message ships here once, remedy included, so the text a reader is
// actually handed is pinned end to end and not only its opening clause.
func Unverified() {} // want `Unverified documents an invariant \("atomically"\) that no test both names and reaches: exercise it from a test whose name carries it, or state the property as description`

// Descriptive returns the number of segments it was given. This documentation
// only describes, and asserts no property at all.
func Descriptive() int { return 0 }

// Forged writes atomically, so no reader observes a partial value.
// Case: a test carries this symbol's name and does nothing whatever with it, so
// it reaches nothing and acquires nothing.
func Forged() {} // want "Forged documents an invariant"

// Mentioned writes atomically, so no reader observes a partial value.
// Case: the test carrying its name spells the symbol without calling it, which
// is the floor of the exemption.
func Mentioned() {}

// Indirect writes atomically, so no reader observes a partial value.
// Case: the test carrying its name reaches it through a helper declared in the
// test file, one hop out from its own body.
func Indirect() {}

// Copied is safe to copy: every field is a value.
// Case: the test carrying its name reaches it through a constructor declared in
// the PACKAGE, so the walk leaves the test files. Same shape as Indirect, on
// the type path rather than the function one.
type Copied struct{ retries int }

// NewCopied builds a Copied.
func NewCopied() Copied { return Copied{retries: 3} }

// Chained writes atomically, so no reader observes a partial value.
// Case: the test carrying its name reaches it two hops out, through a helper
// that calls another helper. A single hop leaves this reported.
func Chained() {}

// Cycled writes atomically, so no reader observes a partial value.
// Case: the helpers between the test and this symbol call each other, so the
// expansion meets a name it has already expanded. Without the visited set the
// walk does not terminate.
func Cycled() {}

// Configured writes atomically, so no reader observes a partial value.
// Case: the test carrying its name drives a package-level value, and the only
// spelling of this symbol sits in that value's initialiser rather than in any
// function body.
func Configured() {}

// Driver is the package-level value whose initialiser reaches Configured.
var Driver = configure()

// configure is where the mention sits.
func configure() int { Configured(); return 0 }

// Handle is safe to copy: it holds one word.
// Case: the test carrying its name spells no such type anywhere, because a Go
// type name lives in declarations rather than in bodies. It calls a constructor
// whose SIGNATURE is the only place this name appears.
type Handle int

// OpenHandle is the constructor whose result type is the mention.
func OpenHandle() Handle { return 0 }

// Held is safe to copy: it holds no reference.
// Case: the test carrying its name builds the struct that CARRIES one, so this
// name appears only in a field declaration. Same mechanism as Handle, on the
// field path rather than the signature one.
type Held struct{}

// Carrier is the struct whose field declaration is the mention.
type Carrier struct{ held Held }

// Split writes atomically, so no reader observes a partial value.
// Case: one test carries the name and a DIFFERENT test reaches the symbol. Two
// halves in two tests are not the exemption, and a corpus that pooled a file's
// identifiers would be unable to tell this apart from the real thing.
func Split() {} // want "Split documents an invariant"

// Unnamed writes atomically, so no reader observes a partial value.
// Case: a test reaches this symbol under a name that does not carry it.
func Unnamed() {} // want "Unnamed documents an invariant"

// blind writes atomically, so no reader observes a partial value.
// Case: the only test carrying its name sits in the EXTERNAL test package and
// reaches nothing at all. A package clause ending _test is free to write and
// acquires no verification, so it excuses nothing.
func blind() {} // want "blind documents an invariant"

// unseen writes atomically, so no reader observes a partial value.
// Case: blind's sibling. Its test is external too and equally unable to spell
// an unexported name, but it drives the exported entry whose body reaches this
// one. Same claim, same shape, same test package, opposite verdict, and the
// only difference between them is whether the naming test reaches the symbol.
func unseen() {}

// Reveal is the exported entry an external test drives to reach unseen.
func Reveal() { unseen() }

// escrowed writes atomically, so no reader observes a partial value.
// Case: unseen's mirror image, in the file that CAN import this package. The
// only test naming it writes `a.Hold()` where `a` is a LOCAL of a type declared
// in that same test file, spelled like the import. The import makes the
// package-qualified route possible and does not make a spelling of it the
// package: the local shadows the import at that selection, so its selected half
// is a member and must not expand this package's own Hold. Crediting it on the
// spelling alone silences this claim for the cost of naming a variable, in the
// only files where the qualified route exists at all.
func escrowed() {} // want "escrowed documents an invariant"

// Hold is the package-level function whose body a shadowed qualifier would
// wrongly reach. Its own documentation only describes.
func Hold() { escrowed() }

// Run is the fleet's mandated domain entry point, and its name is also the
// method the subtest idiom calls on *testing.T. Its own documentation only
// describes, so it is the collision target rather than a subject.
func Run() { staged() }

// staged never drops a record.
// Case: the only test naming it writes `t.Run(...)` and nothing else, so the
// sole route here is the SELECTED half of a method call on a VALUE. A selection
// from a value is a member of that value, not a declaration of this package, so
// it must not expand into what the package's own Run writes down.
func staged() {} // want "staged documents an invariant"

// Store is the value whose method a test drives its subject through.
type Store struct{}

// Save is Store's method, and the only route to stored.
func (s Store) Save() { stored() }

// stored writes atomically, so no reader observes a partial value.
// Case: staged's sibling and the other side of the same boundary. The test
// naming it reaches it through `s.Save()` — a selection from a value whose
// selected half the package really does declare, as a METHOD — so that half
// denotes a declaration this probe holds and expanding it is sound.
func stored() {}

// Len never returns a negative number.
// Case: the only test naming it spells this name once, as the selected half of
// a method call on a value of ANOTHER package's type. This package declares no
// method of that name, so the selection denotes nothing here — neither a
// declaration to expand nor a mention to credit — and the claim stands.
func Len() int { return 0 } // want "Len documents an invariant"

// Shadowed never returns nil.
// Case: the test naming it writes `a.Load()`, where `a` is a LOCAL spelled like
// this package. An internal test file does not import its own package and Go
// has no spelling for one referring to itself, so that `a` is a value and its
// selected half is a member. Crediting it on the spelling alone would expand
// the package-level Load and silence this claim, which is the defect the whole
// rule exists to close, arriving through the back door.
func Shadowed() {} // want "Shadowed documents an invariant"

// Load is the package-level function whose body a shadowing local would
// wrongly reach. Its own documentation only describes.
func Load() { Shadowed() }

// Loader is the type whose method the shadowing local really holds.
type Loader struct{}

// Load is Loader's method, and it reaches nothing.
func (l Loader) Load() {}

// released is safe to copy: it holds no reference.
// Case: `Sealed` is spelled twice by one test — bare, and as the selected half
// of a selection from a value — and this package declares BOTH a function and a
// method of that name. The two are different lookups, so a walk whose visited
// set remembers only the name expands whichever it meets first and drops the
// other, leaving this reachable only through the one it dropped.
func released() {}

// Sealed is the package-level function that reaches released.
func Sealed() { released() }

// Seal is the type carrying the method namesake.
type Seal struct{}

// Sealed is Seal's method, which reaches nothing and exists to collide.
func (s Seal) Sealed() {}

// internalHelper writes atomically, and nothing names it. Unexported symbols
// are IN scope: a property claimed on an unexported helper is still a property
// nothing tests.
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

// MB is always a whole number of bytes.
// Case: the naming half is keyed on the symbol's LENGTH as well as its
// spelling, and every other symbol here is at least three characters, so a rule
// widened to excuse short symbols would change no verdict anywhere else. A test
// below reaches this const under a name that does not carry it, so only the
// naming half stands between the widening and a silence.
const MB = 1 << 20 // want "MB documents an invariant"

// Widened is never reordered.
// Case: the naming half is keyed on the test's NAME, so a disjunct crediting
// some prefix of it — an "integration" convention an author writes freely — is
// a forgeable marker that adds no statement. The only test reaching this symbol
// carries such a prefix and does not carry the symbol's name.
func Widened() {} // want "Widened documents an invariant"

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
