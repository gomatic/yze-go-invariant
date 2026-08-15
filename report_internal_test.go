package invariant

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMessageNamesTheRemedy pins the diagnostic's text, which the corpus cannot
// read and which is as much a part of the rule as the position. A finding whose
// remedy an author has to read the analyzer's source to learn leaves exactly
// one move — the baseline — so the message says what to do about the claim as
// well as what is wrong with it, and it offers both legitimate answers: verify
// the property, or stop asserting one.
func TestMessageNamesTheRemedy(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		`Replace documents an invariant ("atomically") that no test both names and reaches: `+
			`exercise it from a test whose name carries it, or state the property as description`,
		fmt.Sprintf(message, "Replace", "atomically"))
}
