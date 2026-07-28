package a // want "Unverified documents an invariant \\(\"atomically\"\\) that no test names" "internalHelper documents an invariant \\(\"atomically\"\\) that no test names" "Cache documents an invariant \\(\"safe to\"\\) that no test names" "Limit documents an invariant \\(\"always\"\\) that no test names" "Registry documents an invariant \\(\"never\"\\) that no test names"

import "testing"

// TestVerified names the Verified symbol, so its documented claim is satisfied.
func TestVerified(t *testing.T) { Verified() }

// TestDescriptive names a symbol whose documentation asserts nothing.
func TestDescriptive(t *testing.T) {
	if Descriptive() != 0 {
		t.Fatal("Descriptive must start at zero")
	}
}
