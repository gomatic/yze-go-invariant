package invariant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsNamedByMatchesCaseInsensitively pins the symbol-to-test correspondence:
// the NAME half is matched with the symbol's case folded away, while the USE
// half is an exact identifier match, because Go identifiers are case sensitive
// and SetHead and sethead are different symbols.
func TestIsNamedByMatchesCaseInsensitively(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	corpus := testCorpus{
		{name: "testossetheadrenameerrorpreserveshead", uses: usedNames{"SetHead": true}, scope: internalTest},
		{name: "testparse", uses: usedNames{"Parse": true, "Materialize": true}, scope: internalTest},
	}
	want.True(isNamedBy("SetHead", corpus))
	want.True(isNamedBy("Parse", corpus))
	want.False(isNamedBy("Materialize", corpus), "used by a test that does not name it")
	want.False(isNamedBy("sethead", corpus), "the use half matches the identifier exactly")
}

// TestIsNamedByRequiresOneTestToDoBoth is the forgery guard. A test NAME is free
// to write and acquires none of the verification the exemption exists for, so a
// name with no use never counts — and neither does a use under some other
// test's name, because the two halves must meet in the same test.
func TestIsNamedByRequiresOneTestToDoBoth(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.False(isNamedBy("Replace", nil), "no tests to consult exempts nothing")
	want.False(isNamedBy("Replace", testCorpus{}),
		"a package whose only test file holds Benchmarks and Examples has tests but an empty corpus")

	forged := testCorpus{{name: "testreplace", uses: usedNames{}, scope: internalTest}}
	want.False(isNamedBy("Replace", forged), "an empty test named for the symbol acquires nothing")

	split := testCorpus{
		{name: "testreplace", uses: usedNames{"Store": true}, scope: internalTest},
		{name: "testsomethingelse", uses: usedNames{"Replace": true}, scope: internalTest},
	}
	want.False(isNamedBy("Replace", split), "one test names it, another uses it, neither is the exemption")

	whole := testCorpus{{name: "testreplaceisatomic", uses: usedNames{"Replace": true}, scope: internalTest}}
	want.True(isNamedBy("Replace", whole), "one test both names and uses it")
}

// TestVerifiesCountsANameOnlyAsNothing pins the claim verifies makes: a name on
// its own NEVER counts, in either direction. A test named for the symbol that
// mentions nothing is a name and no acquisition, and a test that mentions the
// symbol under a name not carrying it is not about that symbol.
func TestVerifiesCountsANameOnlyAsNothing(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.False(testFunc{name: "testreplace", uses: usedNames{}, scope: internalTest}.verifies("Replace"),
		"the name alone acquires nothing, so it never counts")
	want.False(testFunc{name: "testreplace", uses: nil, scope: internalTest}.verifies("Replace"),
		"a body-less test mentions nothing, so its name alone never counts either")
	elsewhere := testFunc{name: "testsomethingelse", uses: usedNames{"Replace": true}, scope: internalTest}
	want.False(elsewhere.verifies("Replace"), "a mention under an unrelated name is not this test's subject")

	both := testFunc{name: "testreplaceisatomic", uses: usedNames{"Replace": true}, scope: internalTest}
	want.True(both.verifies("Replace"), "one test doing both is the whole exemption")
}

// TestBlindToIsExactlyWhatGoForbids pins the one place the mention is not
// required, and its two edges. An external test package cannot write an
// unexported name, so demanding one there demands the impossible; it CAN write
// an exported name through the qualifier, and an internal test can write either,
// so neither of those is excused.
func TestBlindToIsExactlyWhatGoForbids(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	external := testFunc{name: "testcanonical", uses: usedNames{}, scope: externalTest}
	want.True(external.blindTo("canonical"), "an external test cannot see an unexported name")
	want.False(external.blindTo("Canonical"), "it can see an exported one, through the qualifier")

	internal := testFunc{name: "testcanonical", uses: usedNames{}, scope: internalTest}
	want.False(internal.blindTo("canonical"), "an internal test can write either")
	want.False(internal.blindTo("Canonical"))

	want.True(external.verifies("canonical"), "so the name is the only evidence Go permits")
	want.False(internal.verifies("canonical"), "while inside the package the mention is still required")
}
