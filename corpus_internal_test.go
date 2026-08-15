package invariant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsNamedByMatchesCaseInsensitively pins the symbol-to-test correspondence:
// the NAME half is matched with the symbol's case folded away, while the REACH
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
	want.False(isNamedBy("Materialize", corpus), "reached by a test that does not name it")
	want.False(isNamedBy("sethead", corpus), "the reach half matches the identifier exactly")
}

// TestIsNamedByRequiresOneTestToDoBoth is the forgery guard. A test NAME is free
// to write and acquires none of the verification the exemption exists for, so a
// name with no reach never counts — and neither does a reach under some other
// test's name, because the two halves must meet in the same test.
func TestIsNamedByRequiresOneTestToDoBoth(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.False(isNamedBy("Replace", nil), "no tests to consult exempts nothing")
	want.False(isNamedBy("Replace", testCorpus{}),
		"a package whose only test file holds Benchmarks and Examples has tests but an empty corpus")

	forged := testCorpus{{name: "testreplace", uses: usedNames{}}}
	want.False(isNamedBy("Replace", forged), "an empty test named for the symbol acquires nothing")

	split := testCorpus{
		{name: "testreplace", uses: usedNames{"Store": true}},
		{name: "testsomethingelse", uses: usedNames{"Replace": true}},
	}
	want.False(isNamedBy("Replace", split), "one test names it, another reaches it, neither is the exemption")

	whole := testCorpus{{name: "testreplaceisatomic", uses: usedNames{"Replace": true}}}
	want.True(isNamedBy("Replace", whole), "one test both names and reaches it")
}

// TestVerifiesCountsANameOnlyAsNothing pins the claim verifies makes: a name on
// its own NEVER counts, in either direction. A test named for the symbol that
// reaches nothing is a name and no acquisition, and a test that reaches the
// symbol under a name not carrying it is not about that symbol.
func TestVerifiesCountsANameOnlyAsNothing(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.False(testFunc{name: "testreplace", uses: usedNames{}}.verifies("Replace"),
		"the name alone acquires nothing, so it never counts")
	want.False(testFunc{name: "testreplace", uses: nil}.verifies("Replace"),
		"a body-less test reaches nothing, so its name alone never counts either")
	elsewhere := testFunc{name: "testsomethingelse", uses: usedNames{"Replace": true}}
	want.False(elsewhere.verifies("Replace"), "a reach under an unrelated name is not this test's subject")

	both := testFunc{name: "testreplaceisatomic", uses: usedNames{"Replace": true}}
	want.True(both.verifies("Replace"), "one test doing both is the whole exemption")
}

// TestReachedFollowsEveryCalleeTransitively pins what a test reaches: what it
// spells itself, and what the functions it names spell, and what THEY name, to
// any depth. A test that drives its subject through a helper has touched that
// subject, and a rule that stopped at the first hop would report it.
func TestReachedFollowsEveryCalleeTransitively(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bodies := declarations{
		"outer": usedNames{"inner": true},
		"inner": usedNames{"Chained": true},
	}
	got := bodies.reached(usedNames{"outer": true, "Direct": true})

	want.True(got["Direct"], "what the test spells itself is reached")
	want.True(got["outer"], "the helper it names is reached")
	want.True(got["inner"], "and the helper that helper names")
	want.True(got["Chained"], "and the subject two hops out")
	want.False(got["Absent"], "nothing else is")
	want.Empty(bodies.reached(usedNames{}), "a test that spells nothing reaches nothing")
	want.Empty(bodies.reached(nil), "and neither does one with no body")
}

// TestReachedTerminatesOnMutualRecursion is the walk's cycle guard, written
// from what the guard prevents: two helpers that call each other are ordinary
// Go, and an expansion that does not remember where it has been never returns.
// This test hangs rather than fails if the visited set goes.
func TestReachedTerminatesOnMutualRecursion(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bodies := declarations{
		"ping": usedNames{"pong": true},
		"pong": usedNames{"ping": true, "Cycled": true},
	}
	got := bodies.reached(usedNames{"ping": true})

	want.True(got["Cycled"], "the subject past the cycle is still reached")
	want.Len(got, 3, "and each name in the cycle is expanded once")
}

// TestDeclarationsPoolNamesakes pins that this probe keys a declaration by NAME, having
// no types to key it by anything better: a method and a function sharing a name
// contribute to one entry rather than replacing each other, so a test reaches
// what either of them spells.
func TestDeclarationsPoolNamesakes(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bodies := declarations{}
	bodies.add("Run", usedNames{"Replace": true})
	bodies.add("Run", usedNames{"Delete": true})
	bodies.merge(declarations{"Run": usedNames{"Insert": true}, "Other": usedNames{"Skip": true}})

	want.Equal(usedNames{"Replace": true, "Delete": true, "Insert": true}, bodies["Run"])
	want.Equal(usedNames{"Skip": true}, bodies["Other"], "a name nothing else carries stands alone")
}

// TestExpandedCreditsEachTestWithWhatItReaches pins the seam: the corpus a
// symbol is looked up in holds reached sets, not the tests' own bodies.
func TestExpandedCreditsEachTestWithWhatItReaches(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	corpus := testCorpus{{name: "testchained", uses: usedNames{"outer": true}}}
	bodies := declarations{"outer": usedNames{"Chained": true}}

	want.True(isNamedBy("Chained", expanded(corpus, bodies)), "reached through the helper")
	want.False(isNamedBy("Chained", corpus), "and not before the expansion")
	want.Empty(expanded(nil, bodies), "no tests expand to no corpus")
}
