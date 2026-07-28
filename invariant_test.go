package invariant_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/analysis/analysistest"

	invariant "github.com/gomatic/yze-go-invariant"
)

// TestDocumentedInvariantsMustBeNamedByATest pins the whole contract against
// the fixture: claims on functions, types, consts, and vars are reported when
// no test names the symbol, while a claim whose symbol IS named by a test,
// purely descriptive documentation, an unexported symbol, and an undocumented
// symbol are all left alone.
func TestDocumentedInvariantsMustBeNamedByATest(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), invariant.Analyzer, "a")
}

// TestRegistrationIsWellFormed pins the yze wiring: the rule id the probe
// reports under, and that the registration carries this package's analyzer.
func TestRegistrationIsWellFormed(t *testing.T) {
	t.Parallel()

	assert.NoError(t, invariant.Registration.Validate())
	assert.Equal(t, "yze/invariant", invariant.Registration.RuleID())
	assert.Same(t, invariant.Analyzer, invariant.Registration.Analyzer)
}
