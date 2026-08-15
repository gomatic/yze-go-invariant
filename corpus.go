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
// spells, and the ones spelled by every declaration reachable from it.
type usedNames map[symbolName]bool

// spelled is what one node writes down, split by what each spelling can DENOTE.
// A name written bare, or qualified by this package's own name, denotes a
// declaration of this package. A name written as the selected half of any other
// qualified reference denotes a member of the value it is selected from — a
// field, which writes nothing, or a method, whose body it does write — or a
// symbol of another package, which this package does not declare at all.
//
// The split exists because pooling the two is unsound and was measured to be:
// `t.Run("case", ...)` is the house subtest idiom and `func Run(ctx, logger,
// Config, args...)` is this fleet's mandated domain entry point, so a walk that
// takes every selected half as a package-level name credits every test in every
// such package with reaching whatever the entry point touches.
type spelled struct {
	plain    usedNames
	selected usedNames
}

// newSpelled is an empty spelling set.
func newSpelled() spelled {
	return spelled{plain: usedNames{}, selected: usedNames{}}
}

// absorb folds another node's spellings into these, each into its own half.
func (written spelled) absorb(other spelled) {
	written.plain.absorb(other.plain)
	written.selected.absorb(other.selected)
}

// reference is one name together with what its spelling can denote, which is
// what decides where the walk looks the name up.
type reference struct {
	name     symbolName
	isMember bool
}

// testFunc is one test function as this probe reads it: the identifiers it
// reaches, which say whether it touches a symbol at all, and the name, which
// says which symbol it is ABOUT.
type testFunc struct {
	uses usedNames
	name testName
}

// seededTest is one test function before its reach is expanded: its name, and
// what its own body spells.
type seededTest struct {
	seed spelled
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

// writes is what each declared name writes down, keyed by that name.
type writes map[symbolName]spelled

// declarations is what a package's declarations write down, in two tables
// because a name's SPELLING decides which of them it can reach. A bare or
// self-qualified name reaches a package-level declaration; a name selected from
// a value reaches a method, and nothing else — a field writes nothing and
// another package's symbol is not declared here.
//
// Both tables are keyed by NAME rather than by identity because this probe has
// no type information: it reads two test packages and the package under
// analysis as syntax, so two methods sharing a name share an entry and what
// they write down is pooled.
type declarations struct {
	plain   writes
	methods writes
}

// newDeclarations is an empty pair of tables.
func newDeclarations() declarations {
	return declarations{plain: writes{}, methods: writes{}}
}

// absorb folds every name in other into these.
func (uses usedNames) absorb(other usedNames) {
	for name := range other {
		uses[name] = true
	}
}

// add records what one declaration writes down, unioning into any entry the
// same name already has.
func (table writes) add(name symbolName, written spelled) {
	into, isSeen := table[name]
	if !isSeen {
		into = newSpelled()
		table[name] = into
	}
	into.absorb(written)
}

// merge folds another table into this one.
func (table writes) merge(other writes) {
	for name, written := range other {
		table.add(name, written)
	}
}

// merge folds another file's declarations into these, table by table.
func (decls declarations) merge(other declarations) {
	decls.plain.merge(other.plain)
	decls.methods.merge(other.methods)
}

// lookup is what one reference denotes, and what that writes down.
//
// A name spelled as a declaration always denotes something: this probe has no
// types, so the package's Replace is indistinguishable from a local variable,
// label or field of that name, which is the standing limitation the doc comment
// declares. A name selected from a VALUE denotes only a method this package
// declares — a selection matching no method is a field or another package's
// symbol, which writes nothing here and names nothing here.
func (decls declarations) lookup(ref reference) (spelled, bool) {
	if !ref.isMember {
		return decls.plain[ref.name], true
	}
	written, isMethod := decls.methods[ref.name]
	return written, isMethod
}

// reached expands what a test's own body spells through what each name it
// spells writes down, and through theirs, until nothing new is found. That is
// what makes the exemption a statement about the test rather than about its
// spelling: a test that drives its subject through a helper, a constructor or
// an exported entry point has touched it, and the one-hop form of this walk
// recovers almost none of those.
//
// The visited set is the walk's cycle guard rather than a clause of the rule:
// two helpers that call each other are ordinary Go, and expanding one through
// the other without remembering where the walk has been does not terminate. It
// is keyed by REFERENCE rather than by name because the same name reached both
// ways is two different lookups.
func (decls declarations) reached(seed spelled) usedNames {
	out := usedNames{}
	seen := map[reference]bool{}
	pending := referencesIn(seed)
	for len(pending) > 0 {
		last := len(pending) - 1
		ref := pending[last]
		pending = pending[:last]
		if seen[ref] {
			continue
		}
		seen[ref] = true
		written, isDenoted := decls.lookup(ref)
		if !isDenoted {
			continue
		}
		out[ref.name] = true
		pending = append(pending, referencesIn(written)...)
	}
	return out
}

// referencesIn is a spelling set as a list, so the walk can consume it as a
// stack, each name carrying what its spelling can denote.
func referencesIn(written spelled) []reference {
	out := make([]reference, 0, len(written.plain)+len(written.selected))
	for name := range written.plain {
		out = append(out, reference{name: name})
	}
	for name := range written.selected {
		out = append(out, reference{name: name, isMember: true})
	}
	return out
}

// testCorpus is a package's test functions, across both of its test packages,
// against which a symbol is looked for.
type testCorpus []testFunc

// testSeeds are a package's test functions as read off disk, before the walk.
type testSeeds []seededTest

// expanded replaces each test's own spellings with everything it reaches.
func expanded(seeds testSeeds, decls declarations) testCorpus {
	out := make(testCorpus, 0, len(seeds))
	for _, fn := range seeds {
		out = append(out, testFunc{name: fn.name, uses: decls.reached(fn.seed)})
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
