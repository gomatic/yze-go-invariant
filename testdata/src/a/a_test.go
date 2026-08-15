package a

import (
	"strings"
	"testing"
)

// TestVerified names the Verified symbol and calls it in its own body, so its
// documented claim is satisfied.
func TestVerified(t *testing.T) { Verified() }

// TestForged spells the Forged symbol in its name and never uses it. A name is
// free to write and acquires none of the verification the exemption exists for,
// so it must not silence Forged's claim.
func TestForged(t *testing.T) {}

// TestMentioned names Mentioned and refers to it without calling it, which is
// the floor of the exemption and is silent by design.
func TestMentioned(t *testing.T) { _ = Mentioned }

// TestIndirect names Indirect and reaches it through a helper, so the subject
// is spelled one hop out from this body.
func TestIndirect(t *testing.T) { callIndirect() }

// callIndirect is the one hop, and it is where the mention sits.
func callIndirect() { Indirect() }

// TestChained names Chained and reaches it two hops out.
func TestChained(t *testing.T) { outerHop() }

// outerHop is the first hop and spells no subject.
func outerHop() { innerHop() }

// innerHop is the second hop, and it is where the mention sits.
func innerHop() { Chained() }

// TestCycled names Cycled and reaches it through two helpers that call each
// other, so the expansion meets a name it has already expanded.
func TestCycled(t *testing.T) { ping() }

// ping calls pong.
func ping() { pong() }

// pong calls ping back, and spells the subject.
func pong() { ping(); Cycled() }

// TestConfigured names Configured and reaches it through a package-level
// value, whose initialiser is the only place the subject is spelled.
func TestConfigured(t *testing.T) { _ = Driver }

// TestHandleIsOneWord names Handle and spells no such type: the constructor it
// calls names the type in its result, which is where Go keeps type names.
func TestHandleIsOneWord(t *testing.T) { _ = OpenHandle() }

// TestHeldTravelsInACarrier names Held and spells no such type: the struct it
// builds names the type in a field declaration.
func TestHeldTravelsInACarrier(t *testing.T) { _ = Carrier{} }

// TestConcurrentWritersNeverObserveAMix uses Unnamed but does not carry its
// name, so it does not silence Unnamed's claim.
func TestConcurrentWritersNeverObserveAMix(t *testing.T) { Unnamed() }

// TestDescriptive names a symbol whose documentation asserts nothing.
func TestDescriptive(t *testing.T) {
	if Descriptive() != 0 {
		t.Fatal("Descriptive must start at zero")
	}
}

// TestCopiedIsSafeToCopy exercises Copied through its constructor, so the walk
// has to leave the test files to find the type's name in NewCopied's body.
func TestCopiedIsSafeToCopy(t *testing.T) {
	first := NewCopied()
	second := first
	second.retries = 9
	if first.retries != 3 {
		t.Fatal("a copy shared state with its original")
	}
}

// TestSplit carries Split's name and reaches nothing.
func TestSplit(t *testing.T) {}

// TestWritersObserveNoMix spells Split under a name that does not carry it. The
// two halves of the exemption are present in the file and in no single test.
func TestWritersObserveNoMix(t *testing.T) { Split() }

// TestStagedKeepsEveryRecord names staged and writes the house subtest idiom.
// It reaches staged only if `t.Run` is mistaken for the package's own Run, so
// this test verifies nothing and must not silence the claim.
func TestStagedKeepsEveryRecord(t *testing.T) { t.Run("case", func(t *testing.T) {}) }

// TestStoredIsWrittenAtomically names stored and reaches it through a method
// call on a value, which is the selected half that MUST still count: Save is a
// method this package declares, so the selection denotes a declaration.
func TestStoredIsWrittenAtomically(t *testing.T) {
	var s Store
	s.Save()
}

// TestLenIsNeverNegative names Len and spells it only as the selected half of
// a call on a strings.Builder, whose Len this package does not declare.
func TestLenIsNeverNegative(t *testing.T) {
	var b strings.Builder
	_ = b.Len()
}

// TestShadowedIsNeverNil names Shadowed and writes `a.Load()`, where `a` is a
// local spelled like this package. An internal test file cannot import its own
// package, so that spelling is a value and Load is its member, not the
// package's.
func TestShadowedIsNeverNil(t *testing.T) {
	var a Loader
	a.Load()
}

// TestReleasedIsSafeToCopy names released and spells Sealed twice: bare, which
// is the package function that reaches the subject, and selected from a value,
// which is the method namesake that reaches nothing. One test, two lookups.
func TestReleasedIsSafeToCopy(t *testing.T) {
	var s Seal
	s.Sealed()
	Sealed()
}
