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

// TestADirectiveDoesNotMoveTheFileOrItsDirectory pins both places this probe
// decides a file's identity against the name the go tool compiled rather than
// the one `//line` tells the position machinery to report.
//
// Package forgeline is the per-declaration half, in both directions: ordinary
// source claiming a test name is still reported, and a real test file claiming a
// source name is still spared. Package linedir is the whole-package half: the
// directive sits in the file the directory is derived from, so under a
// position-derived directory the scan finds no tests at all and every claim in
// the package goes silent — including two files carrying no marker.
func TestADirectiveDoesNotMoveTheFileOrItsDirectory(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), invariant.Analyzer, "forgeline", "linedir")
}

// TestTheTestFileMatcherReadsBothEdgesOfItsLiteral pins the file-NAME dimension,
// which every other fixture here holds constant. Package filename sits at each
// edge of "_test.go" and beside it; the package in the directory named
// "inside_test.go" puts the literal inside the PATH rather than at its end,
// which is the escape a substring match opens and no ordinary filename can
// reach.
func TestTheTestFileMatcherReadsBothEdgesOfItsLiteral(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), invariant.Analyzer, "filename", "inside_test.go")
}

// TestRegistrationIsWellFormed pins the yze wiring: the rule id the probe
// reports under, and that the registration carries this package's analyzer.
func TestRegistrationIsWellFormed(t *testing.T) {
	t.Parallel()

	assert.NoError(t, invariant.Registration.Validate())
	assert.Equal(t, "yze/invariant", invariant.Registration.RuleID())
	assert.Same(t, invariant.Analyzer, invariant.Registration.Analyzer)
}
