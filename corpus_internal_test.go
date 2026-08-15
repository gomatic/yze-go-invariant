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

	bodies := newDeclarations()
	bodies.plain.add("outer", plainly("inner"))
	bodies.plain.add("inner", plainly("Chained"))
	got := bodies.reached(plainly("outer", "Direct"))

	want.True(got["Direct"], "what the test spells itself is reached")
	want.True(got["outer"], "the helper it names is reached")
	want.True(got["inner"], "and the helper that helper names")
	want.True(got["Chained"], "and the subject two hops out")
	want.False(got["Absent"], "nothing else is")
	want.Empty(bodies.reached(newSpelled()), "a test that spells nothing reaches nothing")
	want.Empty(bodies.reached(spelled{}), "and neither does one with no body")
}

// TestLookupDenotesADeclarationAlwaysAndAMemberOnlyAsAMethod pins the one
// question the walk asks of every name, in both directions and at its boundary.
// A name spelled as a declaration denotes one whether or not the package
// declares it — this probe has no types, so it cannot tell the package's Cap
// from a local of that name, and reporting a claim because a name is unknown
// would be a finding about the walk. A name selected from a value denotes only
// a method: a selection matching none is a field or another package's symbol,
// and neither is declared here.
func TestLookupDenotesADeclarationAlwaysAndAMemberOnlyAsAMethod(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	decls := newDeclarations()
	decls.plain.add("Run", plainly("staged"))
	decls.methods.add("Save", plainly("stored"))

	written, isDenoted := decls.lookup(reference{name: "Run"})
	want.True(isDenoted, "a bare name denotes a declaration")
	want.Equal(usedNames{"staged": true}, written.plain, "and writes down what that declaration writes")

	_, isDenoted = decls.lookup(reference{name: "Absent"})
	want.True(isDenoted, "a bare name this package never declares still denotes one, having no types to say otherwise")

	written, isDenoted = decls.lookup(reference{name: "Save", isMember: true})
	want.True(isDenoted, "a selection matching a method denotes that method")
	want.Equal(usedNames{"stored": true}, written.plain, "and writes down its body")

	_, isDenoted = decls.lookup(reference{name: "Run", isMember: true})
	want.False(isDenoted, "the same name selected from a value is not the package's Run")

	_, isDenoted = decls.lookup(reference{name: "retries", isMember: true})
	want.False(isDenoted, "and a selection matching no method at all is a field, which declares nothing here")
}

// TestReachedResolvesASelectionAgainstMethodsOnly is the discrimination this
// walk exists to make, cased in both directions. The selected half of a
// selection from a VALUE denotes a member of that value: a method, whose body
// it reaches, or a field, which declares nothing here. It never denotes a
// package-level declaration — `t.Run(...)` is the house subtest idiom and
// `func Run(...)` is this fleet's mandated entry point, and pooling the two
// credits every such test with everything the entry point touches.
func TestReachedResolvesASelectionAgainstMethodsOnly(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bodies := newDeclarations()
	bodies.plain.add("Run", plainly("staged"))
	bodies.methods.add("Save", plainly("stored"))

	fromValue := bodies.reached(selecting("Run", "Save", "retries"))
	want.False(fromValue["Run"], "a selection from a value is not the package's Run")
	want.False(fromValue["staged"], "so it reaches nothing that Run writes")
	want.True(fromValue["Save"], "a selection matching a method denotes that method")
	want.True(fromValue["stored"], "and reaches what its body writes")
	want.False(fromValue["retries"], "a selection matching no method is a field, and denotes nothing here")

	fromPackage := bodies.reached(plainly("Run"))
	want.True(fromPackage["Run"], "the same name spelled bare, or qualified by this package, is the declaration")
	want.True(fromPackage["staged"], "and reaches what it writes")
}

// TestReachedTerminatesOnMutualRecursion is the walk's cycle guard, written
// from what the guard prevents: two helpers that call each other are ordinary
// Go, and an expansion that does not remember where it has been never returns.
// This test hangs rather than fails if the visited set goes.
func TestReachedTerminatesOnMutualRecursion(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bodies := newDeclarations()
	bodies.plain.add("ping", plainly("pong"))
	bodies.plain.add("pong", plainly("ping", "Cycled"))
	got := bodies.reached(plainly("ping"))

	want.True(got["Cycled"], "the subject past the cycle is still reached")
	want.Len(got, 3, "and each name in the cycle is expanded once")
}

// TestReachedRemembersHowANameWasSpelled pins the visited set's key. The same
// name reached both as a declaration and as a selection from a value is two
// different lookups in two different tables, so a walk keyed by name alone
// would drop whichever it met second.
func TestReachedRemembersHowANameWasSpelled(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bodies := newDeclarations()
	bodies.plain.add("Close", plainly("flushed"))
	bodies.methods.add("Close", plainly("released"))

	both := newSpelled()
	both.absorb(plainly("Close"))
	both.absorb(selecting("Close"))
	got := bodies.reached(both)

	want.True(got["flushed"], "what the package's own Close writes")
	want.True(got["released"], "and what the method of that name writes")
}

// TestDeclarationsPoolNamesakes pins that this probe keys a declaration by NAME, having
// no types to key it by anything better: a method and a function sharing a name
// contribute to one entry rather than replacing each other, so a test reaches
// what either of them spells.
func TestDeclarationsPoolNamesakes(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bodies := newDeclarations()
	bodies.plain.add("Run", plainly("Replace"))
	bodies.plain.add("Run", plainly("Delete"))
	other := newDeclarations()
	other.plain.add("Run", plainly("Insert"))
	other.methods.add("Save", plainly("Skip"))
	bodies.merge(other)

	want.Equal(usedNames{"Replace": true, "Delete": true, "Insert": true}, bodies.plain["Run"].plain)
	want.Equal(usedNames{"Skip": true}, bodies.methods["Save"].plain, "a merge carries each table into its own")
	want.NotContains(bodies.plain, symbolName("Save"), "and a method never lands among the declarations")
}

// TestExpandedCreditsEachTestWithWhatItReaches pins the seam: the corpus a
// symbol is looked up in holds reached sets, not the tests' own bodies.
func TestExpandedCreditsEachTestWithWhatItReaches(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	seeds := testSeeds{{name: "testchained", seed: plainly("outer")}}
	bodies := newDeclarations()
	bodies.plain.add("outer", plainly("Chained"))

	want.True(isNamedBy("Chained", expanded(seeds, bodies)), "reached through the helper")
	want.False(isNamedBy("Chained", testCorpus{{name: "testchained", uses: usedNames{"outer": true}}}),
		"and not before the expansion")
	want.Empty(expanded(nil, bodies), "no tests expand to no corpus")
}

// plainly is a spelling set of bare names, the way a body spells them when it
// writes no qualified reference at all.
func plainly(names ...symbolName) spelled {
	written := newSpelled()
	for _, name := range names {
		written.plain[name] = true
	}
	return written
}

// selecting is a spelling set of names written as the selected half of a
// selection from a value.
func selecting(names ...symbolName) spelled {
	written := newSpelled()
	for _, name := range names {
		written.selected[name] = true
	}
	return written
}
