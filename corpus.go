// This file holds the corpus vocabulary: what a package's tests are, reduced
// to the two things this probe can read off them, and the rule that decides
// whether they add up to an exemption.
//
// It is separate from the probe because it is the seam between the two halves —
// scan.go builds a corpus off disk, invariant.go consults one — and because
// keeping each file to one thing is what lets the 1:1 test-layout rule give
// each of them its own test file.

package invariant

import (
	"go/ast"
	"slices"
	"strings"
)

// testName is one test function's name, lower-cased, in which a symbol's own
// name is looked for.
type testName string

// usedNames are the identifiers one test function's body mentions.
type usedNames map[symbolName]bool

// testScope is which of a package's two test packages a test file belongs to,
// which is what decides whether it is able to write an unexported name at all.
type testScope string

const (
	externalTest testScope = "external"
	internalTest testScope = "internal"
)

// testFunc is one test function as this probe reads it: the identifiers its
// body mentions, which say whether it touches a symbol at all, the name, which
// says which symbol it is ABOUT, and the scope, which says what it was ABLE to
// write.
type testFunc struct {
	uses  usedNames
	name  testName
	scope testScope
}

// verifies reports whether this one test both names the symbol and mentions it.
// The conjunction is the whole exemption: a name is free to write and acquires
// nothing, so a name never counts on its own, and a mention under some other
// test's name is not this test's subject.
//
// The naming half is a SUBSTRING test rather than a word match, so a short
// symbol is named by any test whose spelling happens to contain it — "set"
// inside TestOffsetBoundsAreInclusive. Requiring the mention stops that from
// silencing a claim by itself, since the unrelated test has no reason to write
// the symbol; it does not make the match precise, and is not an argument that
// it should stay imprecise.
func (fn testFunc) verifies(symbol symbolName) bool {
	return fn.names(symbol) && (fn.uses[symbol] || fn.blindTo(symbol))
}

// names reports whether the test's own name carries the symbol's.
func (fn testFunc) names(symbol symbolName) bool {
	return strings.Contains(string(fn.name), strings.ToLower(string(symbol)))
}

// blindTo reports whether this test could not have written the symbol's name
// whatever it did, because an external test package cannot see an unexported
// symbol. Demanding a mention there would demand the impossible: the claim
// would report for as long as it existed, and the author's only answer would be
// to move the test into the package — a worse design, chosen to satisfy a probe.
// So where Go permits no mention, the name stands as the only evidence there is,
// on the same reasoning that leaves a package with no tests to the coverage
// gate: "no test names this symbol" and "no test COULD name it" are different
// problems with different owners.
func (fn testFunc) blindTo(symbol symbolName) bool {
	return fn.scope == externalTest && !ast.IsExported(string(symbol))
}

// testCorpus is a package's test functions, across both of its test packages,
// against which a symbol is looked for.
type testCorpus []testFunc

// isNamedBy reports whether some ONE test both names the symbol and uses it.
// Both halves must hold in the SAME test: a test named for the symbol that
// never mentions it, and a test that mentions it under an unrelated name, are
// each half of an exemption and neither is the exemption.
func isNamedBy(symbol symbolName, tested testCorpus) bool {
	return slices.ContainsFunc(tested, func(fn testFunc) bool { return fn.verifies(symbol) })
}
