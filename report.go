// This file holds the reporting half: which of a pass's documented symbols
// become findings, and how they are emitted.
//
// It is separate from the package's wiring because it is a separate concern —
// invariant.go declares the analyzer and states the rule, this applies it — and
// because keeping each file to one thing is what lets the 1:1 test-layout rule
// give each of them its own test file.

package invariant

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

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

// fileOf is the path of the file containing n, as the go tool selected it
// rather than as the file describes itself.
//
// fset.Position applies `//line` directives, which are ordinary compiled source,
// so reading the adjusted name would let a production file write
// `//line zz_test.go:1` and have every claim it declares skipped as a test's —
// this rule switched off for real, compiled, shipped code by one comment line
// that appears in no configuration file and carries no marker. It is wrong in
// the other direction too: a real `_test.go` writing `//line prod.go:1` has its
// own helpers judged as production code and reported. token.File.Name is what
// the go tool compiled, and no directive rewrites it.
func fileOf(pass *analysis.Pass, n ast.Node) fileName {
	return fileName(pass.Fset.File(n.Pos()).Name())
}

// isTest reports whether name is a Go test file, by the same rule the go tool
// applies: the name ENDS in "_test.go", cased as the go tool cases it. Each of
// those three is a case in testdata/src/filename, because each is a widening an
// author reaches for. A suffix widened to "test.go" exempts `httptest.go`, which
// is an ordinary, idiomatic, compiled Go filename; a suffix widened to a
// substring exempts any name merely CONTAINING it, including a package whose
// DIRECTORY carries the spelling; and a suffix folded to lower case exempts
// `upper_Test.go`, which the go tool compiles.
func isTest(name fileName) bool {
	return strings.HasSuffix(string(name), "_test.go")
}
