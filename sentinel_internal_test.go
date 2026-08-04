package invariant

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSentinelNamesExemptsExactlyTheErrorTypedDeclarations names the claim that
// exempting sentinels does not exempt their neighbours.
//
// A const group routinely mixes an error with ordinary values, and the whole
// point of the exemption is that it is decided per NAME by type — not per
// declaration, and not by the identifier's spelling. Exempting the group would
// silence real invariant claims sitting beside a sentinel.
func TestSentinelNamesExemptsExactlyTheErrorTypedDeclarations(t *testing.T) {
	t.Parallel()

	errIdent, plainIdent := ast.NewIdent("ErrThing"), ast.NewIdent("Limit")
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}}

	errType := types.NewNamed(
		types.NewTypeName(token.NoPos, nil, "sentinel", nil), types.Typ[types.String], nil)
	errType.AddMethod(types.NewFunc(token.NoPos, nil, "Error", types.NewSignatureType(
		types.NewParam(token.NoPos, nil, "", errType), nil, nil, nil,
		types.NewTuple(types.NewParam(token.NoPos, nil, "", types.Typ[types.String])), false)))

	info.Defs[errIdent] = types.NewConst(token.NoPos, nil, "ErrThing", errType, nil)
	info.Defs[plainIdent] = types.NewConst(token.NoPos, nil, "Limit", types.Typ[types.Int], nil)

	decl := &ast.GenDecl{Specs: []ast.Spec{
		&ast.ValueSpec{Names: []*ast.Ident{errIdent}},
		&ast.ValueSpec{Names: []*ast.Ident{plainIdent}},
	}}

	got := sentinelNames(info, decl)

	assert.True(t, got["ErrThing"], "an error-typed constant is a sentinel")
	assert.False(t, got["Limit"], "its non-error neighbour in the same group is not")
}

// TestSentinelNamesYieldsNothingWithoutADeclarationOrTypes pins the guards: the
// analyzer also visits function declarations, and a pass may carry no type
// information at all.
func TestSentinelNamesYieldsNothingWithoutADeclarationOrTypes(t *testing.T) {
	t.Parallel()

	assert.Empty(t, sentinelNames(&types.Info{}, &ast.FuncDecl{}), "a func declares no sentinels")
	assert.Empty(t, sentinelNames(nil, &ast.GenDecl{}), "no type info means no judgement")
	assert.False(t, isErrorType(nil), "an untyped node is not an error")
}
