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
// The declarations are in that list because Go keeps type names there. A type
// identifier is almost never spelled inside a body: callers build values with
// composite literals, untyped constants and :=, so a walk over bodies alone can
// never reach a type however thoroughly a test drives one. Measured by hand on
// a sample of twelve findings this change would otherwise have made, seven were
// false and six of the seven were exactly that shape.
//
// The transitive closure is a design decision and not an accident. Stopping at
// the first hop recovers almost nothing: over 279 fleet modules one hop leaves
// 468 findings where the full walk leaves 298, so the shapes this probe was
// wrongly reporting sit further out than a single call.
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
// field of the same name. What the conjunction buys is not that forging is hard
// but that it must be written deliberately, in the body of the test that
// carries the name, where a reader judging the claim will find it. Only a
// mention in a COMMENT fails to silence.
//
// # Documented scope limitations
//
// The walk is keyed by NAME, having no types to key it by anything better, so a
// function and a method sharing a name share an entry and their mentions are
// pooled — and a helper whose name collides with something the test writes for
// another reason is expanded anyway. The closure is unbounded: a test that
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
// A qualified reference counts — store.Replace and a.Replace both reach
// Replace — so a method or another package's symbol sharing the name satisfies
// the reach. There is no type information on the test side.
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
// all: this build reports 298 findings across 61 modules, against 270 across 48
// for the name-only rule it replaces. It adds 28 and silences none of the 270,
// so it is strictly a tightening. Restate that measurement whenever the rule
// changes; a number here describing a build that never shipped is a defect in
// the standard rather than a footnote.
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

// claims are the words a doc comment uses to assert a property rather than to
// describe behaviour. Kept deliberately small: each addition widens the probe's
// noise, and the set earns its place by what it finds.
var claims = []string{
	"atomically", "atomic", "never", "always", "exactly", "unambiguous",
	"guarantee", "byte-identical", "byte-exact", "idempotent", "safe to",
	"deduplicat", "must not", "cannot",
}

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
	tested, hasTests := testedSymbols(readDir, readFile, dir, packageDeclarations(pass.Files))
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

// claimIn returns the first property-asserting phrase in doc, or empty when the
// documentation only describes.
func claimIn(doc docText) claimText {
	lower := strings.ToLower(string(doc))
	for _, claim := range claims {
		if strings.Contains(lower, claim) {
			return claimText(claim)
		}
	}
	return ""
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
