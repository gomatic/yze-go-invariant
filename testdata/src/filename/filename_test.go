package filename

import "testing"

// TestNothingInParticular carries no subject's name, so it is the corpus without
// being an exemption for anything in this package.
func TestNothingInParticular(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
}
