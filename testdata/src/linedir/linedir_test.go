package linedir

import "testing"

// TestScannedIsOneWord names Scanned and reaches it through the constructor
// declared beside it, so this file being read at all is what silences Scanned's
// claim — and its silence is the evidence the corpus came from this directory.
func TestScannedIsOneWord(t *testing.T) {
	if OpenScanned() != 0 {
		t.Fatal("a fresh Scanned is not zero")
	}
}
