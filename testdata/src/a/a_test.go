package a

import "testing"

// TestVerified names the Verified symbol, so its documented claim is satisfied.
func TestVerified(t *testing.T) { Verified() }

// TestForged spells the Forged symbol in its name and never uses it. A name is
// free to write and acquires none of the verification the exemption exists for,
// so it must not silence Forged's claim.
func TestForged(t *testing.T) {}

// TestMentioned names Mentioned and refers to it without calling it, which is
// the floor of the exemption and is silent by design.
func TestMentioned(t *testing.T) { _ = Mentioned }

// TestIndirect names Indirect and reaches it only through a helper, so its own
// body mentions nothing and Indirect's claim is still reported.
func TestIndirect(t *testing.T) { callIndirect() }

// callIndirect is where the mention hides. It is not a test function, so
// nothing it mentions reaches the corpus.
func callIndirect() { Indirect() }

// TestConcurrentWritersNeverObserveAMix uses Unnamed but does not carry its
// name, so it does not silence Unnamed's claim.
func TestConcurrentWritersNeverObserveAMix(t *testing.T) { Unnamed() }

// TestDescriptive names a symbol whose documentation asserts nothing.
func TestDescriptive(t *testing.T) {
	if Descriptive() != 0 {
		t.Fatal("Descriptive must start at zero")
	}
}
