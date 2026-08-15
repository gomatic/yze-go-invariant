// This file holds the test scan: reading a package's directory to learn which
// symbols its tests name, and reading what every declaration writes down — the
// test files' and the analyzed package's — to learn what those tests reach.
//
// It is separate from the probe because it is a separate concern — the probe
// reads doc comments in one compilation unit, this reads files off disk — and
// because keeping each file to one thing is what lets the 1:1 test-layout rule
// give each of them its own test file.

package invariant

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// filePath is the location of one test file being scanned.
type filePath string

// dirPath is the filesystem path of the analyzed package's directory.
type dirPath string

var (
	readDir  dirReader  = osReadDirNames
	readFile fileReader = os.ReadFile
)

// Injected collaborators, so the directory scan is testable without a real tree.
type (
	dirReader  func(dir dirPath) ([]string, error)
	fileReader func(path string) ([]byte, error)
)

// testedSymbols is every test function in the package's directory, each paired
// with everything it reaches, against which a symbol is considered named and
// reached. The package's own declarations are passed in because the pass
// already holds them, and because a test reaches its subject through them as
// readily as through a helper of its own.
// A package with no tests is not this probe's finding to make: the coverage gate
// already requires every package to be tested, and reporting the same gap here
// would add noise without adding information. "No test names this symbol" and
// "there are no tests" are different problems with different owners, so the
// second return distinguishes them.
func testedSymbols(dir dirReader, file fileReader, at dirPath, pkg declarations) (testCorpus, bool) {
	entries, err := dir(at)
	if err != nil {
		return nil, false
	}
	if !anyTest(entries) {
		return nil, false
	}
	var corpus testCorpus
	decls := declarations{}
	decls.merge(pkg)
	for _, entry := range entries {
		if !isTest(fileName(entry)) {
			continue
		}
		tests, declared := testsIn(file, filePath(filepath.Join(string(at), entry)))
		corpus = append(corpus, tests...)
		decls.merge(declared)
	}
	return expanded(corpus, decls), true
}

// anyTest reports whether the directory holds a test file at all.
func anyTest(entries []string) bool {
	return slices.ContainsFunc(entries, func(entry string) bool { return isTest(fileName(entry)) })
}

// testsIn is every test function declared in the file, each carrying its
// lower-cased name and the identifiers its own body mentions, alongside what
// every name the file declares writes down — the helpers a test reaches
// through.
//
// The file is parsed for syntax only: no type information crosses the pass
// boundary, and none is needed to read a function's name or the identifiers it
// writes. An unreadable or unparseable file contributes nothing, so the probe
// fails OPEN — reporting a claim as unverified because a file would not open
// would be a finding about the filesystem.
func testsIn(read fileReader, path filePath) (testCorpus, declarations) {
	src, err := read(string(path))
	if err != nil {
		return nil, nil
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), string(path), src, 0)
	if err != nil {
		return nil, nil
	}
	var out testCorpus
	for _, fn := range funcsIn(parsed) {
		if strings.HasPrefix(fn.Name.Name, "Test") {
			out = append(out, testFunc{
				uses: mentionedIn(fn.Body),
				name: testName(strings.ToLower(fn.Name.Name)),
			})
		}
	}
	return out, declaredIn(parsed)
}

// funcsIn is every function and method a file declares.
func funcsIn(file *ast.File) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			out = append(out, fn)
		}
	}
	return out
}

// declaredIn is what each name a file declares writes down, which is what a
// test reaches by naming it.
func declaredIn(file *ast.File) declarations {
	decls := declarations{}
	for _, decl := range file.Decls {
		switch declared := decl.(type) {
		case *ast.FuncDecl:
			decls.add(symbolName(declared.Name.Name), writtenBy(declared))
		case *ast.GenDecl:
			addSpecs(decls, declared)
		}
	}
	return decls
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
func writtenBy(fn *ast.FuncDecl) usedNames {
	written := mentionedIn(fn.Body)
	written.absorb(identsIn(fn.Type))
	if fn.Recv != nil {
		written.absorb(identsIn(fn.Recv))
	}
	return written
}

// addSpecs records what each name a var, const or type declaration introduces
// writes down: a value's initialiser and declared type, and a type's own
// definition. A package-level value is a body like any other — this fleet
// builds its analyzers as `var Analyzer = newAnalyzer()`, and a test that
// drives Analyzer reaches everything the constructor reaches — and a struct's
// definition is where the types of its fields are spelled.
func addSpecs(decls declarations, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		switch declared := spec.(type) {
		case *ast.ValueSpec:
			addValue(decls, declared)
		case *ast.TypeSpec:
			decls.add(symbolName(declared.Name.Name), identsIn(declared.Type))
		}
	}
}

// addValue keys a value spec's initialisers and declared type under every name
// the spec introduces.
func addValue(decls declarations, value *ast.ValueSpec) {
	written := usedNames{}
	for _, expr := range value.Values {
		written.absorb(identsIn(expr))
	}
	if value.Type != nil {
		written.absorb(identsIn(value.Type))
	}
	for _, name := range value.Names {
		decls.add(symbolName(name.Name), written)
	}
}

// packageDeclarations is what each function the analyzed package declares mentions,
// so a test that drives its subject through the package's own constructor or
// entry point is credited with reaching it.
func packageDeclarations(files []*ast.File) declarations {
	decls := declarations{}
	for _, file := range files {
		decls.merge(declaredIn(file))
	}
	return decls
}

// mentionedIn is every identifier appearing anywhere in a function body,
// including the selected half of a qualified reference, so store.Replace and
// a.Replace both mention Replace.
//
// The nil guard is the walk's, not the rule's: a body-less declaration —
// func TestStub(t *testing.T) with no braces — parses without error, and
// walking its nil body panics. Such a test mentions nothing.
func mentionedIn(body *ast.BlockStmt) usedNames {
	if body == nil {
		return usedNames{}
	}
	return identsIn(body)
}

// identsIn is every identifier anywhere inside one node.
func identsIn(node ast.Node) usedNames {
	mentioned := usedNames{}
	ast.Inspect(node, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok {
			mentioned[symbolName(ident.Name)] = true
		}
		return true
	})
	return mentioned
}

// osReadDirNames lists the entry names of a directory.
func osReadDirNames(dir dirPath) ([]string, error) {
	entries, err := os.ReadDir(string(dir))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// packageDir is the directory holding the package under analysis, or empty when
// the pass carries no files.
func packageDir(fset *token.FileSet, files []*ast.File) dirPath {
	for _, file := range files {
		return dirPath(filepath.Dir(fset.Position(file.Pos()).Filename))
	}
	return ""
}
