// Package invariant provides a go/analysis analyzer that reports documented
// symbols whose doc comment ASSERTS a property no test in the package both
// names and uses.
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
// symbol's name and mentions that symbol in its own body. The name says which
// symbol a test is about; the mention says whether it touches it at all.
//
// The name alone was once enough, and that was a hole rather than a heuristic:
// an empty func TestReplace(t *testing.T) {} silenced a claim outright, having
// run nothing and needing no reference to the subject at all.
//
// The floor is a MENTION rather than an assertion, on purpose, and a mention is
// a SPELLING rather than a reference. This probe reads the test as syntax with
// no types, so it cannot tell an assertion that verifies the claim from one that
// verifies something else, and it cannot tell the package's Replace from a local
// variable, label, field or type of that name. Both are deliberate: demanding
// "an assertion" would move the forgery from one free line to another, and
// resolving identifiers would mean type-checking two test packages the pass
// cannot see. So the exemption is still forgeable, in one line rather than none
// — `func TestReplace(t *testing.T) { var Replace int; _ = Replace }` silences a
// claim — and what the conjunction buys is that the forgery must now be written
// deliberately, in the body of the test that carries the name, where a reader
// looking at the claim will find it.
//
// # Documented scope limitations
//
// Only the test function's OWN body is read, and only as syntax, so a test that
// reaches its subject WITHOUT spelling it has its subject reported. That covers
// a local helper, a constructor, a package-level table of function values, and
// anything reached only from TestMain — all ordinary Go, all reported. Measured:
// 208 of 372 new findings on one fleet sweep were symbols the package's tests do
// mention, somewhere other than in the test carrying the name; the commonest
// single shape is this fleet's own `Run(ctx, ...)` operation exercised by a local
// `run(t)` helper, since the match is case sensitive and run is not Run.
//
// Only a func whose name begins "Test" is a test here. An Example, a Fuzz target
// and a Benchmark all exercise their subject under go test and none of them
// counts, which predates the conjunction and is not narrowed by it.
//
// A qualified reference counts — store.Replace and a.Replace both mention
// Replace — so a method or another package's symbol sharing the name satisfies
// the mention. There is no type information on the test side.
//
// The mention is required only where Go permits one. An external package a_test
// test cannot write an unexported name however thoroughly it exercises it — 148
// of 379 findings on one fleet sweep, none answerable except by moving the test
// inside the package — so for that pair the name stands as the only evidence
// there is. The residual: an unexported symbol is still silenced by an empty
// EXTERNAL test named for it, and by nothing else.
//
// This is a PROBE, not a gate. Its precision is bounded by English rather than
// by Go — the keyword set will match prose that is descriptive rather than
// contractual, and "named and used by a test" is a heuristic for "verified by a
// test". It is built to be read and adjudicated during a quality audit, never
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
const message = "%s documents an invariant (%q) that no test names and uses"

// claims are the words a doc comment uses to assert a property rather than to
// describe behaviour. Kept deliberately small: each addition widens the probe's
// noise, and the set earns its place by what it finds.
var claims = []string{
	"atomically", "atomic", "never", "always", "exactly", "unambiguous",
	"guarantee", "byte-identical", "byte-exact", "idempotent", "safe to",
	"deduplicat", "must not", "cannot",
}

// Analyzer reports documented invariants that no test names and uses.
var Analyzer = &analysis.Analyzer{
	Name:     "invariant",
	Doc:      "reports documented symbols whose doc comment asserts a property that no test in the package names and uses",
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

// finding is one documented invariant that no test both names and uses.
type finding struct {
	symbol symbolName
	claim  claimText
}

// run reports each documented symbol whose documentation asserts a property
// that no test function in the package both names and uses.
func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	tested, hasTests := testedSymbols(readDir, readFile, packageDir(pass.Fset, pass.Files))
	if !hasTests {
		return nil, nil
	}
	report(pass, findings(pass, ins, tested))
	return nil, nil
}

// findings collects every documented invariant declared in a non-test file
// that no test both names and uses, in source order.
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
// comment asserts a property that no test both names and uses.
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
