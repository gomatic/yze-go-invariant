package invariant

import (
	"go/ast"
	"go/types"
)

// sentinelNames are the declaration's symbols whose type is an error.
//
// A sentinel's doc says WHEN it is returned — "ErrCreateOutputDir is returned
// when the output directory cannot be created" — so the claim word belongs to
// the failure being described, not to a property the code upholds. Reading it
// as an invariant asks for a test of the opposite of what the sentinel means,
// and the shape is not rare: six of one repository's eleven findings were
// exactly this, and together they held its gate red.
//
// That a sentinel IS asserted somewhere remains a real requirement. It belongs
// to yze/errtested, which checks each one is matched with errors.Is.
func sentinelNames(info *types.Info, node ast.Node) map[symbolName]bool {
	decl, ok := node.(*ast.GenDecl)
	if !ok || info == nil {
		return nil
	}
	names := map[symbolName]bool{}
	for _, spec := range decl.Specs {
		addSentinels(info, spec, names)
	}
	return names
}

// addSentinels records every name in one spec whose type is an error.
func addSentinels(info *types.Info, spec ast.Spec, into map[symbolName]bool) {
	value, ok := spec.(*ast.ValueSpec)
	if !ok {
		return
	}
	for _, ident := range value.Names {
		if isErrorType(info.TypeOf(ident)) {
			into[symbolName(ident.Name)] = true
		}
	}
}

// isErrorType reports whether t satisfies the builtin error interface.
func isErrorType(t types.Type) bool {
	if t == nil {
		return false
	}
	err, ok := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	return ok && types.Implements(t, err)
}
