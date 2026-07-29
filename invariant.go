// Package invariant provides a go/analysis analyzer that reports documented
// symbols whose doc comment ASSERTS a property no test in the package names.
//
// A doc comment saying a write lands "atomically", that a reconstruction is
// "byte-identical", or that a value is "safe to copy" is a contract claim.
// Nothing in a conventional gate reads it: statement coverage is satisfied by
// executing the code, and a linter checks form rather than English. So a
// property can be advertised in documentation, believed by every reader, and
// verified by nothing.
//
// This is a PROBE, not a gate. Its precision is bounded by English rather than
// by Go — the keyword set will match prose that is descriptive rather than
// contractual, and "named by a test" is a heuristic for "verified by a test".
// It is built to be read and adjudicated during a quality audit, never to block
// a push. In practice the keyword set lands on documented properties that no
// test verifies, alongside claims that do have tests — which is the expected
// behaviour of a probe and precisely why a human or agent decides, not the
// build.
package invariant

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// message is the diagnostic emitted for an unverified documented invariant.
const message = "%s documents an invariant (%q) that no test names"

// claims are the words a doc comment uses to assert a property rather than to
// describe behaviour. Kept deliberately small: each addition widens the probe's
// noise, and the set earns its place by what it finds.
var claims = []string{
	"atomically", "atomic", "never", "always", "exactly", "unambiguous",
	"guarantee", "byte-identical", "byte-exact", "idempotent", "safe to",
	"deduplicat", "must not", "cannot",
}

// Analyzer reports documented invariants that no test names.
var Analyzer = &analysis.Analyzer{
	Name:     "invariant",
	Doc:      "reports documented symbols whose doc comment asserts a property that no test in the package names",
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

// testNames is the corpus of a package's test function names, lower-cased and
// newline separated, against which a symbol is looked for.
type testNames string

// finding is one documented invariant with no test naming its symbol.
type finding struct {
	symbol symbolName
	claim  claimText
}

// run reports each exported symbol whose documentation asserts a property that
// no test function in the package names.
func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	tested, hasTests := testedSymbols(readDir, readFile, packageDir(pass.Fset, pass.Files))
	if !hasTests {
		return nil, nil
	}
	report(pass, findings(pass, ins, tested))
	return nil, nil
}

// findings collects every documented invariant on an exported symbol in a
// non-test file that no test names, in source order.
func findings(pass *analysis.Pass, ins *inspector.Inspector, tested testNames) map[finding]token.Pos {
	found := map[finding]token.Pos{}
	nodes := []ast.Node{(*ast.FuncDecl)(nil), (*ast.GenDecl)(nil)}
	ins.Preorder(nodes, func(n ast.Node) {
		if isTest(fileOf(pass, n)) {
			return
		}
		collect(n, tested, found)
	})
	return found
}

// collect records a finding when n's documentation asserts a property and its
// symbol is not named by any test.
func collect(node ast.Node, tested testNames, into map[finding]token.Pos) {
	symbol, doc := documented(node)
	if symbol == "" || isNamedBy(symbol, tested) {
		return
	}
	if claim := claimIn(doc); claim != "" {
		into[finding{symbol: symbol, claim: claim}] = node.Pos()
	}
}

// documented returns the symbol a declaration introduces and its doc comment.
// An undocumented declaration yields an empty name.
func documented(node ast.Node) (symbolName, docText) {
	switch decl := node.(type) {
	case *ast.FuncDecl:
		return documentedIdent(decl.Name, decl.Doc)
	case *ast.GenDecl:
		return genDeclSymbol(decl)
	}
	return "", ""
}

// genDeclSymbol returns the symbol a type, const, or var declaration
// introduces, taking the first spec's name as the declaration's subject.
func genDeclSymbol(decl *ast.GenDecl) (symbolName, docText) {
	if len(decl.Specs) == 0 {
		return "", ""
	}
	switch spec := decl.Specs[0].(type) {
	case *ast.TypeSpec:
		return documentedIdent(spec.Name, decl.Doc)
	case *ast.ValueSpec:
		return firstValueName(spec, decl.Doc)
	}
	return "", ""
}

// firstValueName returns the first name a value spec declares. Go's grammar
// requires at least one name in a value spec, so there is no empty case here.
func firstValueName(spec *ast.ValueSpec, doc *ast.CommentGroup) (symbolName, docText) {
	return documentedIdent(spec.Names[0], doc)
}

// documentedIdent pairs an identifier with its documentation, yielding an empty
// name when the declaration carries no doc comment.
//
// Unexported symbols are deliberately IN scope. Measured against real
// codebases, the highest-value claims sat on unexported helpers — an "atomic"
// write, an "unambiguous" encoding — and restricting to the exported surface
// missed every one of them while saving only a handful of findings.
func documentedIdent(ident *ast.Ident, doc *ast.CommentGroup) (symbolName, docText) {
	if doc == nil {
		return "", ""
	}
	return symbolName(ident.Name), docText(doc.Text())
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

// isNamedBy reports whether any test function name contains the symbol.
func isNamedBy(symbol symbolName, tested testNames) bool {
	return strings.Contains(string(tested), strings.ToLower(string(symbol)))
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
