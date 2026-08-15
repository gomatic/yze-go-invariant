package invariant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClaimInFindsOnlyAssertions pins that asserting prose is recognised and
// describing prose is not.
func TestClaimInFindsOnlyAssertions(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.Equal(claimText("atomically"), claimIn("Write lands atomically."))
	want.Equal(claimText("safe to"), claimIn("Store is safe to copy."))
	want.Empty(claimIn("Parse returns the parsed document."))
	want.Empty(claimIn(""))
}
