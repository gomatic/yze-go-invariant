package invariant

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDocumentedIgnoresNonDeclarations pins the classifier's fallthrough: a node
// that declares nothing yields no symbol, so it can never become a finding.
func TestDocumentedIgnoresNonDeclarations(t *testing.T) {
	t.Parallel()

	assert.Empty(t, documented(&ast.ReturnStmt{}))
}

// TestGenDeclSymbolsIgnoresEmptyGroups pins the empty declaration group —
// `var ()` is legal Go and introduces no symbol — and the import declaration,
// whose specs introduce no documented symbol either.
func TestGenDeclSymbolsIgnoresEmptyGroups(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.Empty(genDeclSymbols(&ast.GenDecl{}))

	doc := &ast.CommentGroup{List: []*ast.Comment{{Text: "// The imports."}}}
	imports := &ast.GenDecl{Doc: doc, Specs: []ast.Spec{&ast.ImportSpec{}}}
	want.Empty(genDeclSymbols(imports), "an import spec introduces no documented symbol")
}

// TestGenDeclSymbolsReadsEachSpecsOwnDoc pins the grouped-declaration form:
// a doc comment on a member INSIDE a const (...) or var (...) group is read
// and paired with THAT member's name, alongside the declaration's own doc
// paired with the first member.
func TestGenDeclSymbolsReadsEachSpecsOwnDoc(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	declDoc := &ast.CommentGroup{List: []*ast.Comment{{Text: "// The group."}}}
	memberDoc := &ast.CommentGroup{List: []*ast.Comment{{Text: "// second never fails."}}}
	group := &ast.GenDecl{Doc: declDoc, Specs: []ast.Spec{
		&ast.ValueSpec{Names: []*ast.Ident{{Name: "first"}}},
		&ast.ValueSpec{Doc: memberDoc, Names: []*ast.Ident{{Name: "second"}}},
	}}

	got := genDeclSymbols(group)
	want.Len(got, 2)
	want.Equal(symbolName("first"), got[0].symbol, "the declaration's doc speaks for the first member")
	want.Contains(string(got[0].doc), "The group")
	want.Equal(symbolName("second"), got[1].symbol, "a member's own doc speaks for that member")
	want.Contains(string(got[1].doc), "never fails")
}

// TestSubjectOfNamesTypeAndValueSpecs pins the spec classifier: a type spec's
// name, a value spec's first name, and nothing for any other spec kind.
func TestSubjectOfNamesTypeAndValueSpecs(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.Equal("Thing", subjectOf(&ast.TypeSpec{Name: &ast.Ident{Name: "Thing"}}).Name)
	want.Equal("first", subjectOf(&ast.ValueSpec{Names: []*ast.Ident{{Name: "first"}}}).Name)
	want.Nil(subjectOf(&ast.ImportSpec{}))
	want.Nil(subjectOf(&ast.ValueSpec{}), "a nameless value spec names nothing to anchor at")
}

// TestSpecDocReadsTheSpecsOwnComment pins where a grouped member's doc lives:
// on the type or value spec itself, and nowhere for other spec kinds.
func TestSpecDocReadsTheSpecsOwnComment(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	doc := &ast.CommentGroup{List: []*ast.Comment{{Text: "// Doc."}}}
	want.Same(doc, specDoc(&ast.TypeSpec{Doc: doc, Name: &ast.Ident{Name: "Thing"}}))
	want.Same(doc, specDoc(&ast.ValueSpec{Doc: doc, Names: []*ast.Ident{{Name: "first"}}}))
	want.Nil(specDoc(&ast.ImportSpec{}))
}

// TestNamedRequiresADocComment pins that documentation is what brings
// a symbol into scope, and that being unexported does not take it out.
func TestNamedRequiresADocComment(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	doc := &ast.CommentGroup{List: []*ast.Comment{{Text: "// Thing never fails."}}}
	got := named(&ast.Ident{Name: "Thing"}, doc, 0)
	want.Len(got, 1)
	want.Equal(symbolName("Thing"), got[0].symbol)
	want.Contains(string(got[0].doc), "never fails")

	want.Empty(named(&ast.Ident{Name: "Thing"}, nil, 0), "an undocumented symbol makes no claim")

	got = named(&ast.Ident{Name: "thing"}, doc, 0)
	want.Len(got, 1)
	want.Equal(symbolName("thing"), got[0].symbol, "an unexported symbol still makes a claim")
}

// TestClaimInFindsOnlyAssertions pins that asserting prose is recognised and
// describing prose is not.
func TestClaimInFindsOnlyAssertions(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.Equal(claimText("atomically"), claimIn("Write lands atomically."))
	want.Equal(claimText("safe to"), claimIn("Store is safe to copy."))
	want.Empty(claimIn("Parse returns the parsed document."))
	want.Empty(claimIn(""))
}
