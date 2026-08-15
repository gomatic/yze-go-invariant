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
// run nothing and needing no reference to the package's subject at all.
// Requiring the mention makes the cheapest forgery cost the same act as writing
// a shallow test — the symbol must exist, be reachable, and compile there.
//
// The floor is a MENTION rather than an assertion, on purpose. This probe reads
// syntax, so it cannot tell an assertion that verifies the claim from one that
// verifies something else, and demanding "an assertion" would move the forgery
// from one free line to another. A test that names a symbol and merely refers
// to it is therefore silent — the residual heuristic conceded below, and not a
// hole, because acquiring it means writing a test about the symbol.
//
// # Documented scope limitations
//
// Only the test function's OWN body is read, and only as syntax. A test that
// reaches its subject through a helper, or through a constructor that never
// writes the type's own name, mentions nothing itself, so that subject is
// reported: the probe follows neither calls nor types, and errs toward
// reporting, a false positive costing an adjudication where a false negative
// costs the point.
//
// A qualified reference counts — store.Replace and a.Replace both mention
// Replace — so a method or another package's symbol sharing the name satisfies
// the mention. There is no type information on the test side.
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

// documentedSymbol is one symbol a declaration documents: its name, the doc
// comment speaking for it, and the position a finding anchors at.
type documentedSymbol struct {
	symbol symbolName
	doc    docText
	pos    token.Pos
}

// documented returns the symbols a declaration introduces, each paired with
// the doc comment that speaks for it. An undocumented declaration yields
// nothing.
func documented(node ast.Node) []documentedSymbol {
	switch decl := node.(type) {
	case *ast.FuncDecl:
		return named(decl.Name, decl.Doc, decl.Pos())
	case *ast.GenDecl:
		return genDeclSymbols(decl)
	}
	return nil
}

// genDeclSymbols returns the symbols a type, const, or var declaration
// documents: the declaration's own doc paired with the first spec's name, and
// each spec's OWN doc paired with that spec's name — the grouped-declaration
// form, where a member of a const (...) or var (...) group documents itself.
// A claim in a group member's doc is as much a contract claim as one on the
// declaration.
func genDeclSymbols(decl *ast.GenDecl) []documentedSymbol {
	if len(decl.Specs) == 0 {
		return nil
	}
	out := speced(decl.Specs[0], decl.Doc, decl.Pos())
	for _, spec := range decl.Specs {
		out = append(out, specSymbols(spec)...)
	}
	return out
}

// specSymbols pairs a spec's OWN doc comment with its subject, anchored at
// that subject: a claim written on a group member points at the member, not
// at the group. The position comes from the identifier rather than the spec
// so nothing is asked of a spec that introduces no symbol.
func specSymbols(spec ast.Spec) []documentedSymbol {
	ident := subjectOf(spec)
	if ident == nil {
		return nil
	}
	return named(ident, specDoc(spec), ident.Pos())
}

// speced pairs a spec's subject identifier with doc, yielding nothing for a
// spec that introduces no symbol (an import).
func speced(spec ast.Spec, doc *ast.CommentGroup, pos token.Pos) []documentedSymbol {
	ident := subjectOf(spec)
	if ident == nil {
		return nil
	}
	return named(ident, doc, pos)
}

// subjectOf is the identifier a spec declares — a type's name, or a value
// spec's first name. Go's grammar requires at least one name in a value spec,
// so there is no empty case there.
func subjectOf(spec ast.Spec) *ast.Ident {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return s.Name
	case *ast.ValueSpec:
		if len(s.Names) == 0 {
			return nil
		}
		return s.Names[0]
	}
	return nil
}

// specDoc is the doc comment a spec itself carries — present in the grouped
// form, absent (held by the declaration instead) in the ungrouped one.
func specDoc(spec ast.Spec) *ast.CommentGroup {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return s.Doc
	case *ast.ValueSpec:
		return s.Doc
	}
	return nil
}

// named pairs an identifier with the documentation speaking for it, yielding
// nothing when there is no doc comment.
//
// Unexported symbols are deliberately IN scope. Measured against real
// codebases, the highest-value claims sat on unexported helpers — an "atomic"
// write, an "unambiguous" encoding — and restricting to the exported surface
// missed every one of them while saving only a handful of findings.
func named(ident *ast.Ident, doc *ast.CommentGroup, pos token.Pos) []documentedSymbol {
	if doc == nil {
		return nil
	}
	return []documentedSymbol{{symbol: symbolName(ident.Name), doc: docText(doc.Text()), pos: pos}}
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
