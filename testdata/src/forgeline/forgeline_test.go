//line nottest.go:1
package forgeline

import "testing"

// spared writes atomically, so no reader observes a partial value. It is the
// other direction: a real test file that a directive tells the position
// machinery to call nottest.go. It is test code, so the claim on a helper here
// is not a promise to any caller and must not be reported.
func spared() {}

// TestSomethingElse carries no subject's name and reaches nothing, so it is the
// corpus without being an exemption for anything in this package.
func TestSomethingElse(t *testing.T) {
	spared()
	if testing.Short() {
		t.Skip()
	}
}
