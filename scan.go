// This file holds the test-name scan: reading a package's directory to learn
// which symbols its tests name.
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

// testedSymbols is the concatenated, lower-cased names of every test function
// in the pass, against which a symbol is considered named.
// A package with no tests is not this probe's finding to make: the coverage gate
// already requires every package to be tested, and reporting the same gap here
// would add noise without adding information. "No test names this symbol" and
// "there are no tests" are different problems with different owners, so the
// second return distinguishes them.
func testedSymbols(dir dirReader, file fileReader, at dirPath) (testNames, bool) {
	var names strings.Builder
	entries, err := dir(at)
	if err != nil {
		return "", false
	}
	if !anyTest(entries) {
		return "", false
	}
	for _, entry := range entries {
		if !isTest(fileName(entry)) {
			continue
		}
		appendTestNames(file, filePath(filepath.Join(string(at), entry)), &names)
	}
	return testNames(names.String()), true
}

// anyTest reports whether the directory holds a test file at all.
func anyTest(entries []string) bool {
	return slices.ContainsFunc(entries, func(entry string) bool { return isTest(fileName(entry)) })
}

// appendTestNames adds the lower-cased name of every test function in the file.
//
// The file is parsed for syntax only: no type information crosses the pass
// boundary, and none is needed to read a function's name. An unreadable or
// unparseable file contributes nothing, so the probe fails OPEN — reporting a
// claim as unverified because a file would not open would be a finding about the
// filesystem.
func appendTestNames(read fileReader, path filePath, into *strings.Builder) {
	src, err := read(string(path))
	if err != nil {
		return
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), string(path), src, 0)
	if err != nil {
		return
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && strings.HasPrefix(fn.Name.Name, "Test") {
			_, _ = into.WriteString(strings.ToLower(fn.Name.Name) + "\n")
		}
	}
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
