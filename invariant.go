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
// A name written bare, or qualified by the package under analysis — its own
// package name, or an alias the file binds to its import path — refers to a
// DECLARATION of that package. `a.Reveal()` is why that half matters: an
// external `package a_test` cannot spell an unexported name at all, so a
// package-qualified call is its only route to one.
//
// A name written as the selected half of any OTHER qualified reference is
// selected from a value, so it refers to a MEMBER: a method, whose body it
// reaches, or a field, which declares nothing here. It never refers to a
// package-level declaration.
//
// Pooling those two was a soundness defect, and this suite is why. Its own
// mandated domain signature is `func Run(ctx, logger, Config, args...)` and
// `t.Run("case", func(t *testing.T) {...})` is the house subtest idiom, so a
// walk taking every selected half as a package-level name credits every subtest
// in such a package with everything the entry point touches. Reproduced on
// `gomatic/template.cli`'s own greet command, where a claim on `compose`,
// reachable only from `Run`, is silenced by a table test that never mentions
// it; 78 fleet packages carry that shape. Deriving the package's own spelling
// from the last element of its import PATH does not work either — this fleet
// puts package `authority` at `.../go-authority`.
//
// Crediting the method half is not decoration: dropping it adds 208 findings
// over 279 fleet modules, on symbols their tests drive through a method —
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
// 476 findings where the full walk leaves 300, and stopping at the test's own
// body leaves 657, so the shapes this probe was wrongly reporting sit further
// out than a single call.
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
// field of the same name. Only a mention in a COMMENT fails to silence.
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
//   - the OWN-BODY rule that briefly replaced it reported 495 across 94. This
//     build silences 200 of those and adds 5, so against THAT baseline it is a
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
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

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

// findings collects every documented invariant declared in a non-test file
// that no test both names and reaches, in source order.
func findings(pass *analysis.Pass, ins *inspector.Inspector, tested testCorpus) map[finding]token.Pos {
	found := map[finding]token.Pos{}
	nodes := []ast.Node{(*ast.FuncDecl)(nil), (*ast.GenDecl)(nil)}
	ins.Preorder(nodes, func(n ast.Node) {
		if isTest(fileOf(pass, n)) {
			return
		}
		collect(pass.TypesInfo, n, tested, found)
	})
	return found
}

// collect records a finding for each of n's documented symbols whose doc
// comment asserts a property that no test both names and reaches.
func collect(info *types.Info, node ast.Node, tested testCorpus, into map[finding]token.Pos) {
	sentinels := sentinelNames(info, node)
	for _, item := range documented(node) {
		if sentinels[item.symbol] || isNamedBy(item.symbol, tested) {
			continue
		}
		if claim := claimIn(item.doc); claim != "" {
			into[finding{symbol: item.symbol, claim: claim}] = item.pos
		}
	}
}

// sorted returns the findings in source order so output is deterministic.
func sorted(found map[finding]token.Pos) []finding {
	out := make([]finding, 0, len(found))
	for item := range found {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return found[out[i]] < found[out[j]] })
	return out
}

// report emits one diagnostic per finding, anchored at the DECLARATION carrying
// the claim. That is where a reader has to go to judge it, and it is available
// now that the reporting pass no longer has to be one holding test files.
func report(pass *analysis.Pass, found map[finding]token.Pos) {
	for _, item := range sorted(found) {
		pass.Reportf(found[item], message, item.symbol, item.claim)
	}
}

// fileOf is the path of the file containing n.
func fileOf(pass *analysis.Pass, n ast.Node) fileName {
	return fileName(pass.Fset.Position(n.Pos()).Filename)
}

// isTest reports whether name is a Go test file.
func isTest(name fileName) bool {
	return strings.HasSuffix(string(name), "_test.go")
}
