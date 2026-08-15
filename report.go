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

// fileOf is the path of the file containing n.
func fileOf(pass *analysis.Pass, n ast.Node) fileName {
	return fileName(pass.Fset.Position(n.Pos()).Filename)
}

// isTest reports whether name is a Go test file.
func isTest(name fileName) bool {
	return strings.HasSuffix(string(name), "_test.go")
}
