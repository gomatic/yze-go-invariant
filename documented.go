// This file holds the declaration reader: which symbols a declaration
// introduces, and which doc comment speaks for each of them.
//
// It is separate from the probe because it is a separate concern — this reads
// Go's declaration grammar, the probe reads English — and because keeping each
// file to one thing is what lets the 1:1 test-layout rule give each of them its
// own test file.

package invariant

import (
	"go/ast"
	"go/token"
)

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
