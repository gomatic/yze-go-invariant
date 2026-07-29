package a

import "testing"

// TestVerified names the Verified symbol, so its documented claim is satisfied.
func TestVerified(t *testing.T) { Verified() }

// TestDescriptive names a symbol whose documentation asserts nothing.
func TestDescriptive(t *testing.T) {
	if Descriptive() != 0 {
		t.Fatal("Descriptive must start at zero")
	}
}
