// Package invariant provides a go/analysis analyzer that reports documented
// symbols whose doc comment ASSERTS a property no test in the package both
// names and reaches.
//
// A doc comment saying a write lands "atomically", that a reconstruction is
// "byte-identical", or that a value is "safe to copy" is a contract claim.
// Nothing in a conventional gate reads it: statement coverage is satisfied by
// executing the code, and a linter checks form rather than English. So a
// property can be advertised in documentation, believed by every reader, and
// verified by nothing.
//
// # The one exemption, and the floor under it
//
// A claim counts as verified when some ONE test function both carries the
// symbol's name and REACHES that symbol. The name says which symbol a test is
// about; the reach says whether it touches it at all.
//
// The name alone was once enough, and that was a hole rather than a heuristic:
// an empty func TestReplace(t *testing.T) {} silenced a claim outright, having
// run nothing and needing no reference to the subject at all.
//
// # What reaching means
//
// A test reaches every identifier its own body writes, and then everything
// written down by each name it reaches, transitively and to any depth, across
// both of the package's test packages and the package under analysis. What a
// name writes down is:
//
//   - for a function or method, its body, its signature and its receiver;
//   - for a var or const, its initialiser and its declared type;
//   - for a type, its definition — a struct's fields and their types.
//
// So a test that drives its subject through a helper, a constructor, an
// exported entry point or `var Analyzer = newAnalyzer()` has reached it.
//
// # What a spelling can denote, which is the qualified-reference rule
//
// A name written bare, or qualified by a name the file's own IMPORT of the
// package under analysis binds — its alias, or the package's declared name
// where there is no alias — refers to a DECLARATION of that package. Any OTHER
// selected half is selected from a value, so it refers to a MEMBER: a method,
// whose body it reaches, or a field, which declares nothing here.
//
// `a.Reveal()` is why the first half matters: an external `package a_test`
// cannot spell an unexported name at all, so a package-qualified call is its
// only route to one. Pooling the two was a soundness defect, and this suite is
// why: its mandated domain signature is `func Run(ctx, logger, Config, args...)`
// and `t.Run("case", ...)` is the house subtest idiom, so a walk taking every
// selected half as a package-level name credits every subtest in such a package
// with everything the entry point touches. 78 directories across the 279 fleet
// modules declare a package-level `func Run` and hold a test writing `t.Run`.
// The control is `gomatic/template.cli`'s greet command with a claim PLANTED on
// `compose` and a subtest-only test named for it — neither is in the shipped
// repo, which reports nothing under any of these builds — where the parent is
// silent and this build reports.
//
// Requiring the IMPORT is NECESSARY and is not the whole guard, and reading it
// as the whole guard was a hole. A file that does not import the package cannot
// package-qualify it — Go has no spelling for a package referring to itself — so
// inside `package store` the `store` in `store.Get()` is a VALUE, and matching
// the package's name without the import is wrong in both directions at once:
// a local named for the package would make a method call expand the package's
// function of that name, reopening this defect, and the same local would stop a
// genuine method call from reaching the method's body. 43 internal test files
// across the fleet bind such a local and select on it. Deriving the name from
// the import PATH instead does not work either — this fleet puts package
// `authority` at `.../go-authority`.
//
// But the qualified route exists ONLY in an external test file, and that file
// does import the package, so there the import is satisfied by any local of the
// same spelling: `store := fake{}; store.Fetch()` silenced a claim that
// `f := fake{}; f.Fetch()` reports, one identifier apart, in the only files the
// guard was written for. So the qualifier must also be a name the file does not
// itself BIND, which go/parser decides by the language's own scope rules rather
// than by a guess at which syntax declares a name: it resolves the local and
// leaves the import unresolved. The discrimination is per-BODY, not per-file —
// one external test file can shadow the import in one function and qualify
// through it in the next, and both are read correctly.
//
// Crediting the method half is not decoration: dropping it adds 209 findings
// over the 279 modules, on symbols their tests drive through a method —
// `gomatic/go-authority`'s `canonical` among them, the case the deleted
// unexported carve-out existed for.
//
// The declarations are in that list because Go keeps type names there. A type
// identifier is almost never spelled inside a body: callers build values with
// composite literals, untyped constants and :=, so a walk over bodies alone can
// never reach a type however thoroughly a test drives one. Measured by hand on
// a sample of twelve findings this change would otherwise have made, seven were
// false and six of the seven were exactly that shape.
//
// The transitive closure is a design decision and not an accident. Stopping at
// the first hop recovers almost nothing: over 279 fleet modules one hop leaves
// 478 findings where the full walk leaves 300, and stopping at the test's own
// body leaves 659, so the shapes this probe was wrongly reporting sit further
// out than a single call. (Both are this build with only the walk shortened —
// not the 497 of the own-body rule that briefly shipped, which also carried the
// unexported carve-out this one does not.)
//
// The floor is a REACH rather than an assertion, on purpose, and a reach is a
// SPELLING rather than a reference. This probe reads the tests as syntax with
// no types, so it cannot tell an assertion that verifies the claim from one
// that verifies something else, and it cannot tell the package's Replace from a
// local variable, label, field or type of that name. Both are deliberate:
// demanding "an assertion" would move the forgery from one free line to
// another, and resolving identifiers would mean type-checking two test packages
// the pass cannot see. So the exemption is forgeable in one line —
// `func TestReplace(t *testing.T) { var Replace int; _ = Replace }` silences a
// claim, and so do a t.Skip()ped body, a body inside `if false`, and a struct
// field of the same name. What does NOT silence is a mention in a comment, and
// the selected half of a selection from a value that matches no method this
// package declares — `b.Len()` on a strings.Builder is not this package's Len.
//
// And for most findings the cheapest silence is not one line but NONE. Measured
// over the fleet sweep below, 253 of the 300 findings are on symbols some test
// already reaches — deleting the naming half outright leaves 47 — so they are
// reported only because no test's NAME happens to carry the symbol, and the
// whole forgery is renaming an existing test, which
// adds no line and appears in no body. That is the honest statement of what the
// conjunction buys: it is not that forging is hard, and it is not that a reader
// judging the claim will find the forgery. It is only that both halves have to
// be true of one test at once, which no accident produces.
//
// # Documented scope limitations
//
// The walk is keyed by NAME, having no types to key it by anything better, so
// two methods sharing a name share an entry and their mentions are pooled — and
// a helper whose name collides with something the test writes for another
// reason is expanded anyway. That is a live hole and not only a caveat: a test
// declaring `var capOf int` is credited with reaching everything the package's
// `capOf` writes down, including the `Cap` in its signature, and no harness is
// needed to write it. Splitting the tables by spelling narrows this to
// same-kind collisions; it does not close it, and closing it needs types the
// pass does not have on the test side. The closure is unbounded: a test that
// drives a large entry point reaches most of the package. What bounds the
// exemption is the other half, since a symbol is only ever excused by a test
// whose NAME carries it.
//
// The naming half is a substring match, so a short symbol is named by any test
// whose spelling happens to contain it. The reach half is case SENSITIVE, since
// Go identifiers are: a `run(t)` helper does not reach `Run`.
//
// A BARE name is matched by spelling and nothing else, which is the same missing
// type information stated once more and has one further consequence worth
// naming: a test file that DOT-IMPORTS another package supplies bare spellings
// this package never declared, and they are credited as this package's. An
// external test writing `import . "strings"` and calling `Replace(...)` silences
// a claim on this package's own `Replace`, which it never calls; written
// `strings.Replace(...)` the same test reports it. That route is closed by the
// language for an internal test file, which cannot dot-import a supplier of a
// name its own package declares, and it is closed by policy for this fleet,
// whose lint gate bans dot-imports outright — but nothing here closes it, and
// nothing here can without types.
//
// Only a func whose name begins "Test" is a test here. An Example, a Fuzz target
// and a Benchmark all exercise their subject under go test and none of them
// counts.
//
// A qualified reference counts as far as its spelling allows, per the section
// above: `a.Replace` reaches the package's Replace, `store.Replace` reaches a
// METHOD named Replace and nothing else, and a selection matching no method is
// a field and reaches nothing. There is no type information on the test side,
// so `store` is not known to be a value of this package's type and `a` is
// matched by spelling.
//
// The walk follows names, so a symbol reached only through a route that writes
// no name is still reported. Reflection is the standing example: encoding/json
// dispatches MarshalJSON without any caller spelling it, and one finding in the
// hand-adjudicated sample above is precisely that. A string-keyed registry, a
// plugin table loaded at run time and a build-tag-selected implementation are
// the same shape.
//
// The corpus is the DIRECTORY, not the build. Every *_test.go file beside the
// package is parsed for syntax, with no build constraints evaluated, so a test
// file that `go test` never compiles and never runs still supplies both halves.
// That is a known escape, shared with every analyzer in this suite that reasons
// about test files, and it is not settleable here: it needs the tag set the
// gate itself runs under.
//
// # What it costs
//
// Measured on 2026-08-15 over the 279 fleet modules this suite is developed
// against, with 39 of them failing to load and therefore measured as nothing at
// all, this build reports 300 findings across 61 modules. Against each rule it
// succeeds, and every comparison matters:
//
//   - the NAME-ONLY rule reported 270 across 48. This build adds 30 and
//     silences none of them, so against that baseline it is strictly a
//     tightening.
//   - the OWN-BODY rule that briefly replaced it reported 497 across 94. This
//     build silences 202 of those and adds 5, so against THAT baseline it is a
//     large loosening, and a maintainer reading only the first bullet would
//     have it backwards.
//   - the build that pooled every selected half with the package's own names
//     reported 298 across 61. This build adds 2 and silences none, because that
//     defect is LATENT: 78 fleet packages carry the `func Run` / `t.Run` shape,
//     and today only two of them also carry a claim on a symbol nothing else
//     reaches. A fleet delta of 2 is not evidence that the defect is small.
//
// Restate all of them whenever the rule changes; a number here describing a
// build that never shipped is a defect in the standard rather than a footnote.
// They are also a measurement of a MOMENT, not a constant: the fleet is a set of
// live working trees, and the own-body binary measured 495 earlier the same day
// and 497 twice in a row afterwards, unchanged binary. Every number above was
// taken from one back-to-back run on 2026-08-15.
//
// RE-MEASURED after the qualifier guard stopped accepting a spelling the file
// itself binds: 262 fleet modules, this build against the build before it, 300
// findings across 61 modules under BOTH and zero silenced in either direction,
// with 47 reach-only and 253 name-alone reproducing exactly. The delta is not
// evidence that the change is small and must not be read as one: the fleet holds
// no external test file that shadows its own import at a selection reaching a
// claimed symbol, so the dimension is held constant and no result over it could
// have differed. The evidence is the constructed module, where one identifier
// decides the verdict, and it reported +1 as its own control in the same sweep.
//
// This is a PROBE, not a gate. Its precision is bounded by English rather than
// by Go — the keyword set will match prose that is descriptive rather than
// contractual, and "named and reached by a test" is a heuristic for "verified
// by a test". It is built to be read and adjudicated during a quality audit, never
// to block a push. In practice the keyword set lands on documented properties
// that no test verifies, alongside claims that do have tests — which is the
// expected behaviour of a probe and precisely why a human or agent decides, not
// the build.
package invariant

import (
	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// message is the diagnostic emitted for an unverified documented invariant.
const message = "%s documents an invariant (%q) that no test both names and reaches: " +
	"exercise it from a test whose name carries it, or state the property as description"

// Analyzer reports documented invariants that no test names and reaches.
var Analyzer = &analysis.Analyzer{
	Name:     "invariant",
	Doc:      "reports documented symbols whose doc comment asserts a property that no test in the package names and reaches",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// Registration declares this analyzer to the yze framework.
var Registration = goyze.Registration{
	Name:       "invariant",
	Categories: []goyze.Category{"tests", "documentation"},
	URL:        "https://docs.gomatic.dev/yze/invariant",
	Analyzer:   Analyzer,
}

// fileName is a source file's path as recorded in the pass's file set.
type fileName string

// symbolName is the identifier of a declared, documented symbol.
type symbolName string

// claimText is the phrase from a doc comment that asserts a property.
type claimText string

// docText is a declaration's doc comment, as plain text.
type docText string

// finding is one documented invariant that no test both names and reaches.
type finding struct {
	symbol symbolName
	claim  claimText
}

// run reports each documented symbol whose documentation asserts a property
// that no test function in the package both names and reaches.
func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	dir := packageDir(pass.Fset, pass.Files)
	id := packageIdentity{name: packageName(pass.Pkg.Name()), path: packagePath(pass.Pkg.Path())}
	tested, hasTests := testedSymbols(readDir, readFile, dir, id, packageDeclarations(pass.Files, id))
	if !hasTests {
		return nil, nil
	}
	report(pass, findings(pass, ins, tested))
	return nil, nil
}
