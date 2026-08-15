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
// name each IMPORT of that package's path binds, which is the alias where the
// file writes one and the package's own declared name where it does not.
//
// The IMPORT is what makes this POSSIBLE, and it is half the guard; the other
// half is isSelfQualified's, which asks whether the qualifier is a spelling the
// file itself binds. A file that does not import the package cannot
// package-qualify it — Go has no spelling for a package referring to itself, so
// inside `package store` the `store` in `store.Get()` is necessarily a VALUE.
// Matching the package's name without the import gets that backwards in both
// directions at once: it reopens the defect this rule exists to close, since a
// local named for the package makes a method call expand the package's function
// of that name; and it invents a new one, since the same local stops a genuine
// method call from reaching the method's body. 43 internal test files across 279
// fleet modules bind a local spelled like their own package and then select on
// it.
//
// Deriving the name from the import PATH instead of from the package's own
// would be wrong here and was checked: this fleet's convention puts a library's
// package `authority` at the path `github.com/gomatic/go-authority`, so the
// last path element is not what an unaliased import binds.
func selfNamesIn(file *ast.File, id packageIdentity) selfNames {
	names := selfNames{}
	for _, spec := range file.Imports {
		if importedPath(spec) != id.path {
			continue
		}
		names[boundName(spec, id)] = true
	}
	return names
}

// boundName is the name an import of the analyzed package binds: its alias
// where the file writes one, and the package's own declared name where it does
// not, since that is what Go binds for an unaliased import.
func boundName(spec *ast.ImportSpec, id packageIdentity) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	return string(id.name)
}

// importedPath is the path an import spec names.
//
// The unquote error is discarded rather than guarded because it carries nothing
// a caller could act on: strconv.Unquote returns the empty string on every
// failure, and the empty string is not a path any package under analysis has,
// so a malformed spec compares unequal exactly as a failure should. A guard
// here would branch to the value the fall-through already produces, which is
// dead code wearing the shape of care.
func importedPath(spec *ast.ImportSpec) packagePath {
	path, _ := strconv.Unquote(spec.Path.Value)
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
	if isSelfQualified(expr.X, self) {
		found.plain[name] = true
		return found
	}
	found.selected[name] = true
	return found
}

// isSelfQualified reports whether a qualified reference's qualifier is the
// package under analysis rather than a value.
//
// The import is NECESSARY and is not sufficient, and reading it as sufficient
// was a hole. An INTERNAL test file cannot import its own package, so its
// selfNames is empty and a local spelled like the package is harmless there —
// but the qualified route exists only in an EXTERNAL test file, where the import
// IS present, so a name test satisfies self[...] for any local of that spelling.
// One local named for the import then makes every selection on it expand the
// package's declaration of that name: `store := fake{}; store.Fetch()` silenced a
// claim that `f := fake{}; f.Fetch()` reports, with nothing else changed.
//
// Which spelling a qualifier BINDS is a scoping question, and go/parser answers
// it: the resolver records an Object on every identifier it resolves within the
// file, and an import name is deliberately not among them, so a resolved
// qualifier is a local, a parameter, a receiver or a file-level declaration and
// an unresolved one is the import. That is exact rather than approximate — it is
// the language's own scope rules rather than a guess at which syntax declares a
// name — and it is available because scan.go parses test files with resolution
// on. TestIsSelfQualifiedSeparatesTheImportFromAShadowingLocal pins that
// dependency, so a toolchain that stopped resolving would fail the suite rather
// than quietly reopen this.
//
// A package-level declaration of the analyzed package's own name in the same
// file cannot be the qualifier here, because Go refuses to compile a file that
// both imports a name and declares it.
func isSelfQualified(qualifier ast.Expr, self selfNames) bool {
	ident, isIdent := qualifier.(*ast.Ident)
	return isIdent && self[ident.Name] && ident.Obj == nil
}
