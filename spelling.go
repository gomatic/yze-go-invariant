// This file holds the spelling reader: what one node writes down, and what each
// name it writes can DENOTE — a declaration of the package under analysis, or a
// member of some value. That distinction is the whole of the qualified-reference
// rule, and it is decided from syntax alone because no type information reaches
// the test side of this probe.
//
// It is separate from the directory scan because it is a separate concern — the
// scan reads files off disk, this reads the syntax it gets back — and because
// keeping each file to one thing is what lets the 1:1 test-layout rule give
// each of them its own test file.

package invariant

import (
	"go/ast"
	"strconv"
)

// packageName is the name clause of the package under analysis.
type packageName string

// packagePath is the import path of the package under analysis.
type packagePath string

// packageIdentity is how a file may spell the package under analysis: its name,
// which is what an unaliased import of it binds, and its import path, which is
// what an alias is bound to. It is what tells a package qualifier — the only
// qualifier whose selected half denotes a declaration of this package — from a
// value.
type packageIdentity struct {
	name packageName
	path packagePath
}

// selfNames are the names one file uses for the package under analysis.
type selfNames map[string]bool

// selfNamesIn is every name this file uses for the package under analysis: the
// package's own name, which is what an unaliased import of it binds and what
// its own files call it, plus any ALIAS this file binds that import path to.
//
// Deriving the name from the import PATH instead would be wrong here and was
// checked: this fleet's own convention puts a library's package `authority` at
// the path `github.com/gomatic/go-authority`, so the last path element is not
// the bound name. The package's declared name and an explicit alias are the two
// spellings that are actually decidable from syntax.
func selfNamesIn(file *ast.File, id packageIdentity) selfNames {
	names := selfNames{string(id.name): true}
	for _, spec := range file.Imports {
		if spec.Name != nil && importedPath(spec) == id.path {
			names[spec.Name.Name] = true
		}
	}
	return names
}

// importedPath is the path an import spec names, or empty when it is not a
// quoted string the parser could read.
func importedPath(spec *ast.ImportSpec) packagePath {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return ""
	}
	return packagePath(path)
}

// declaredIn is what each name a file declares writes down, which is what a
// test reaches by naming it.
func declaredIn(file *ast.File, id packageIdentity) declarations {
	self := selfNamesIn(file, id)
	decls := newDeclarations()
	for _, decl := range file.Decls {
		switch declared := decl.(type) {
		case *ast.FuncDecl:
			addFunc(decls, declared, self)
		case *ast.GenDecl:
			addSpecs(decls, declared, self)
		}
	}
	return decls
}

// addFunc records what a function or method writes down, in the table its own
// spelling can reach: a method is only ever reached as the selected half of a
// selection from a value, and a function only as a name.
func addFunc(decls declarations, fn *ast.FuncDecl, self selfNames) {
	name := symbolName(fn.Name.Name)
	written := writtenBy(fn, self)
	if fn.Recv != nil {
		decls.methods.add(name, written)
		return
	}
	decls.plain.add(name, written)
}

// writtenBy is everything a function declaration writes down: its body, its
// SIGNATURE, and its receiver.
//
// The signature is not decoration here, it is where Go keeps type names. A type
// identifier is spelled in declarations and almost never inside a body: callers
// build values with composite literals, untyped constants and :=, so a walk
// over bodies alone can never reach a type however thoroughly a test drives it.
// Crediting the signature is what lets a test that calls Parse be credited with
// reaching the Pattern that Parse returns.
func writtenBy(fn *ast.FuncDecl, self selfNames) spelled {
	written := mentionedIn(fn.Body, self)
	written.absorb(identsIn(fn.Type, self))
	if fn.Recv != nil {
		written.absorb(identsIn(fn.Recv, self))
	}
	return written
}

// addSpecs records what each name a var, const or type declaration introduces
// writes down: a value's initialiser and declared type, and a type's own
// definition. A package-level value is a body like any other — this fleet
// builds its analyzers as `var Analyzer = newAnalyzer()`, and a test that
// drives Analyzer reaches everything the constructor reaches — and a struct's
// definition is where the types of its fields are spelled.
func addSpecs(decls declarations, decl *ast.GenDecl, self selfNames) {
	for _, spec := range decl.Specs {
		switch declared := spec.(type) {
		case *ast.ValueSpec:
			addValue(decls, declared, self)
		case *ast.TypeSpec:
			decls.plain.add(symbolName(declared.Name.Name), identsIn(declared.Type, self))
		}
	}
}

// addValue keys a value spec's initialisers and declared type under every name
// the spec introduces.
func addValue(decls declarations, value *ast.ValueSpec, self selfNames) {
	written := newSpelled()
	for _, expr := range value.Values {
		written.absorb(identsIn(expr, self))
	}
	if value.Type != nil {
		written.absorb(identsIn(value.Type, self))
	}
	for _, name := range value.Names {
		decls.plain.add(symbolName(name.Name), written)
	}
}

// packageDeclarations is what each name the analyzed package declares writes
// down, so a test that drives its subject through the package's own constructor
// or entry point is credited with reaching it.
func packageDeclarations(files []*ast.File, id packageIdentity) declarations {
	decls := newDeclarations()
	for _, file := range files {
		decls.merge(declaredIn(file, id))
	}
	return decls
}

// mentionedIn is what a function body spells.
//
// The nil guard is the walk's, not the rule's: a body-less declaration —
// func TestStub(t *testing.T) with no braces — parses without error, and
// walking its nil body panics. Such a test spells nothing.
func mentionedIn(body *ast.BlockStmt, self selfNames) spelled {
	if body == nil {
		return newSpelled()
	}
	return identsIn(body, self)
}

// identsIn is every identifier anywhere inside one node, each recorded by what
// its spelling can denote.
func identsIn(node ast.Node, self selfNames) spelled {
	found := newSpelled()
	ast.Inspect(node, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.SelectorExpr:
			found.absorb(selectedBy(expr, self))
			return false
		case *ast.Ident:
			found.plain[symbolName(expr.Name)] = true
		}
		return true
	})
	return found
}

// selectedBy splits one qualified reference: everything the qualifier itself
// spells, plus the selected half — recorded as a DECLARATION when the qualifier
// is this package, and as a MEMBER otherwise.
//
// This is the whole discrimination. `a.Reveal()` is how an external test
// package reaches an unexported symbol, so its selected half has to keep
// counting; `t.Run(...)` is a method on a value that happens to share a name
// with this fleet's mandated entry point, and crediting it credits every claim
// that entry point transitively touches.
func selectedBy(expr *ast.SelectorExpr, self selfNames) spelled {
	found := identsIn(expr.X, self)
	name := symbolName(expr.Sel.Name)
	if qualifier, isIdent := expr.X.(*ast.Ident); isIdent && self[qualifier.Name] {
		found.plain[name] = true
		return found
	}
	found.selected[name] = true
	return found
}
