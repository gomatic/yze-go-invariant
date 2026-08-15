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
		{name: "testossetheadrenameerrorpreserveshead", uses: usedNames{"SetHead": true}},
		{name: "testparse", uses: usedNames{"Parse": true, "Materialize": true}},
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

	forged := testCorpus{{name: "testreplace", uses: usedNames{}}}
	want.False(isNamedBy("Replace", forged), "an empty test named for the symbol acquires nothing")

	split := testCorpus{
		{name: "testreplace", uses: usedNames{"Store": true}},
		{name: "testsomethingelse", uses: usedNames{"Replace": true}},
	}
	want.False(isNamedBy("Replace", split), "one test names it, another uses it, neither is the exemption")

	whole := testCorpus{{name: "testreplaceisatomic", uses: usedNames{"Replace": true}}}
	want.True(isNamedBy("Replace", whole), "one test both names and uses it")
}

// TestVerifiesCountsANameOnlyAsNothing pins the claim verifies makes: a name on
// its own NEVER counts, in either direction. A test named for the symbol that
// mentions nothing is a name and no acquisition, and a test that mentions the
// symbol under a name not carrying it is not about that symbol.
func TestVerifiesCountsANameOnlyAsNothing(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.False(testFunc{name: "testreplace", uses: usedNames{}}.verifies("Replace"),
		"the name alone acquires nothing, so it never counts")
	want.False(testFunc{name: "testreplace", uses: nil}.verifies("Replace"),
		"a body-less test mentions nothing, so its name alone never counts either")
	want.False(testFunc{name: "testsomethingelse", uses: usedNames{"Replace": true}}.verifies("Replace"),
		"a mention under an unrelated name is not this test's subject")
	want.True(testFunc{name: "testreplaceisatomic", uses: usedNames{"Replace": true}}.verifies("Replace"),
		"one test doing both is the whole exemption")
}
