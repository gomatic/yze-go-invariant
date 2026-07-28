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

	symbol, doc := documented(&ast.ReturnStmt{})

	assert.Empty(t, symbol)
	assert.Empty(t, doc)
}

// TestGenDeclSymbolIgnoresEmptyGroups pins the empty declaration group — `var ()`
// is legal Go and introduces no symbol.
func TestGenDeclSymbolIgnoresEmptyGroups(t *testing.T) {
	t.Parallel()

	symbol, doc := genDeclSymbol(&ast.GenDecl{})

	assert.Empty(t, symbol)
	assert.Empty(t, doc)
}

// TestDocumentedIdentRequiresADocComment pins that documentation is what brings
// a symbol into scope, and that being unexported does not take it out.
func TestDocumentedIdentRequiresADocComment(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	doc := &ast.CommentGroup{List: []*ast.Comment{{Text: "// Thing never fails."}}}
	symbol, text := documentedIdent(&ast.Ident{Name: "Thing"}, doc)
	want.Equal(symbolName("Thing"), symbol)
	want.Contains(string(text), "never fails")

	symbol, _ = documentedIdent(&ast.Ident{Name: "Thing"}, nil)
	want.Empty(symbol, "an undocumented symbol makes no claim")

	symbol, _ = documentedIdent(&ast.Ident{Name: "thing"}, doc)
	want.Equal(symbolName("thing"), symbol, "an unexported symbol still makes a claim")
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

// TestIsNamedByMatchesCaseInsensitively pins the symbol-to-test correspondence.
func TestIsNamedByMatchesCaseInsensitively(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	corpus := testNames("testossetheadrenameerrorpreserveshead\ntestparse\n")
	want.True(isNamedBy("SetHead", corpus))
	want.True(isNamedBy("Parse", corpus))
	want.False(isNamedBy("Materialize", corpus))
}
