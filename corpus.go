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

// usedNames are the identifiers a test function reaches: the ones its own body
// mentions, and the ones mentioned by every function reachable from it.
type usedNames map[symbolName]bool

// testFunc is one test function as this probe reads it: the identifiers it
// reaches, which say whether it touches a symbol at all, and the name, which
// says which symbol it is ABOUT.
type testFunc struct {
	uses usedNames
	name testName
}

// verifies reports whether this one test both names the symbol and reaches it.
// The conjunction is the whole exemption: a name is free to write and acquires
// nothing, so a name never counts on its own, and a mention under some other
// test's name is not this test's subject.
//
// The naming half is a SUBSTRING test rather than a word match, so a short
// symbol is named by any test whose spelling happens to contain it — "set"
// inside TestOffsetBoundsAreInclusive. Requiring the reach stops that from
// silencing a claim by itself, since the unrelated test has no reason to touch
// the symbol; it does not make the match precise, and is not an argument that
// it should stay imprecise.
func (fn testFunc) verifies(symbol symbolName) bool {
	return fn.names(symbol) && fn.uses[symbol]
}

// names reports whether the test's own name carries the symbol's.
func (fn testFunc) names(symbol symbolName) bool {
	return strings.Contains(string(fn.name), strings.ToLower(string(symbol)))
}

// declarations is what each declared name writes down — a function's body,
// signature and receiver, a value's initialiser and declared type, a type's own
// definition — keyed by the name that declares it. It is keyed by NAME rather
// than by identity because this probe has no type information: it reads two
// test packages and the package under analysis as syntax, so a method and a
// function sharing a name share an entry, and what they write down is pooled.
type declarations map[symbolName]usedNames

// absorb folds every name in other into these.
func (uses usedNames) absorb(other usedNames) {
	for name := range other {
		uses[name] = true
	}
}

// add records what one declaration writes down, unioning into any entry the
// same name already has.
func (decls declarations) add(name symbolName, uses usedNames) {
	into, seen := decls[name]
	if !seen {
		into = usedNames{}
		decls[name] = into
	}
	into.absorb(uses)
}

// merge folds another file's declarations into these.
func (decls declarations) merge(other declarations) {
	for name, uses := range other {
		decls.add(name, uses)
	}
}

// reached expands what a test's own body mentions through what each name it
// mentions writes down, and through theirs, until nothing new is found. That is
// what makes the exemption a statement about the test rather than about its
// spelling: a test that drives its subject through a helper, a constructor or
// an exported entry point has touched it, and the one-hop form of this walk
// recovers almost none of those.
//
// The visited set is the walk's cycle guard rather than a clause of the rule:
// two helpers that call each other are ordinary Go, and expanding one through
// the other without remembering where the walk has been does not terminate.
func (decls declarations) reached(seed usedNames) usedNames {
	out := usedNames{}
	pending := toExpand(seed)
	for len(pending) > 0 {
		last := len(pending) - 1
		name := pending[last]
		pending = pending[:last]
		if out[name] {
			continue
		}
		out[name] = true
		pending = append(pending, toExpand(decls[name])...)
	}
	return out
}

// toExpand is a mention set as a list, so the walk can consume it as a stack.
func toExpand(uses usedNames) []symbolName {
	out := make([]symbolName, 0, len(uses))
	for name := range uses {
		out = append(out, name)
	}
	return out
}

// testCorpus is a package's test functions, across both of its test packages,
// against which a symbol is looked for.
type testCorpus []testFunc

// expanded replaces each test's own mentions with everything it reaches.
func expanded(corpus testCorpus, decls declarations) testCorpus {
	out := make(testCorpus, 0, len(corpus))
	for _, fn := range corpus {
		out = append(out, testFunc{name: fn.name, uses: decls.reached(fn.uses)})
	}
	return out
}

// isNamedBy reports whether some ONE test both names the symbol and reaches it.
// Both halves must hold in the SAME test: a test named for the symbol that
// never touches it, and a test that touches it under an unrelated name, are
// each half of an exemption and neither is the exemption.
func isNamedBy(symbol symbolName, tested testCorpus) bool {
	return slices.ContainsFunc(tested, func(fn testFunc) bool { return fn.verifies(symbol) })
}
