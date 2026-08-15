// This file holds the test scan: reading a package's directory off disk to
// learn which test functions it declares and which symbols each of them names.
// What a declaration writes down, and what each name it writes can denote, is
// spelling.go's.
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
func testedSymbols(
	dir dirReader,
	file fileReader,
	at dirPath,
	id packageIdentity,
	pkg declarations,
) (testCorpus, bool) {
	entries, err := dir(at)
	if err != nil {
		return nil, false
	}
	if !anyTest(entries) {
		return nil, false
	}
	var seeds testSeeds
	decls := newDeclarations()
	decls.merge(pkg)
	for _, entry := range entries {
		if !isTest(fileName(entry)) {
			continue
		}
		tests, declared := testsIn(file, filePath(filepath.Join(string(at), entry)), id)
		seeds = append(seeds, tests...)
		decls.merge(declared)
	}
	return expanded(seeds, decls), true
}

// anyTest reports whether the directory holds a test file at all.
func anyTest(entries []string) bool {
	return slices.ContainsFunc(entries, func(entry string) bool { return isTest(fileName(entry)) })
}

// testsIn is every test function declared in the file, each carrying its
// lower-cased name and what its own body spells, alongside what every name the
// file declares writes down — the helpers a test reaches through.
//
// The file is parsed for syntax only: no type information crosses the pass
// boundary, and none is needed to read a function's name or the identifiers it
// writes. An unreadable or unparseable file contributes nothing, so the probe
// fails OPEN — reporting a claim as unverified because a file would not open
// would be a finding about the filesystem.
func testsIn(read fileReader, path filePath, id packageIdentity) (testSeeds, declarations) {
	src, err := read(string(path))
	if err != nil {
		return nil, newDeclarations()
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), string(path), src, 0)
	if err != nil {
		return nil, newDeclarations()
	}
	self := selfNamesIn(parsed, id)
	var out testSeeds
	for _, fn := range funcsIn(parsed) {
		if strings.HasPrefix(fn.Name.Name, "Test") {
			out = append(out, seededTest{
				seed: mentionedIn(fn.Body, self),
				name: testName(strings.ToLower(fn.Name.Name)),
			})
		}
	}
	return out, declaredIn(parsed, id)
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
