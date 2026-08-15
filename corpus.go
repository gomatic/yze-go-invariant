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
	"slices"
	"strings"
)

// testName is one test function's name, lower-cased, in which a symbol's own
// name is looked for.
type testName string

// usedNames are the identifiers one test function's body mentions.
type usedNames map[symbolName]bool

// testFunc is one test function as this probe reads it: the identifiers its
// body mentions, which say whether it touches a symbol at all, and the name,
// which says which symbol it is ABOUT.
type testFunc struct {
	uses usedNames
	name testName
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
	return fn.uses[symbol] && strings.Contains(string(fn.name), strings.ToLower(string(symbol)))
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
