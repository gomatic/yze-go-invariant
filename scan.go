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

// testedSymbols is every test function in the package's directory, each paired
// with the identifiers its body mentions, against which a symbol is considered
// named and used.
// A package with no tests is not this probe's finding to make: the coverage gate
// already requires every package to be tested, and reporting the same gap here
// would add noise without adding information. "No test names this symbol" and
// "there are no tests" are different problems with different owners, so the
// second return distinguishes them.
func testedSymbols(dir dirReader, file fileReader, at dirPath) (testCorpus, bool) {
	entries, err := dir(at)
	if err != nil {
		return nil, false
	}
	if !anyTest(entries) {
		return nil, false
	}
	var corpus testCorpus
	for _, entry := range entries {
		if !isTest(fileName(entry)) {
			continue
		}
		corpus = append(corpus, testsIn(file, filePath(filepath.Join(string(at), entry)))...)
	}
	return corpus, true
}

// anyTest reports whether the directory holds a test file at all.
func anyTest(entries []string) bool {
	return slices.ContainsFunc(entries, func(entry string) bool { return isTest(fileName(entry)) })
}

// testsIn is every test function declared in the file, each carrying its
// lower-cased name and the identifiers its own body mentions.
//
// The file is parsed for syntax only: no type information crosses the pass
// boundary, and none is needed to read a function's name or the identifiers it
// writes. An unreadable or unparseable file contributes nothing, so the probe
// fails OPEN — reporting a claim as unverified because a file would not open
// would be a finding about the filesystem.
func testsIn(read fileReader, path filePath) testCorpus {
	src, err := read(string(path))
	if err != nil {
		return nil
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), string(path), src, 0)
	if err != nil {
		return nil
	}
	scope := scopeOf(packageClause(parsed.Name.Name))
	var out testCorpus
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && strings.HasPrefix(fn.Name.Name, "Test") {
			out = append(out, testFunc{
				uses:  mentionedIn(fn.Body),
				name:  testName(strings.ToLower(fn.Name.Name)),
				scope: scope,
			})
		}
	}
	return out
}

// packageClause is the name a test file declares in its package clause.
type packageClause string

// scopeOf reads a test file's package clause: `package a_test` is the external
// test package, which is unable to see the package's unexported names, and
// anything else is the internal one, which can see them all.
func scopeOf(pkg packageClause) testScope {
	if strings.HasSuffix(string(pkg), "_test") {
		return externalTest
	}
	return internalTest
}

// mentionedIn is every identifier appearing anywhere in a function body,
// including the selected half of a qualified reference, so store.Replace and
// a.Replace both mention Replace.
//
// The nil guard is the walk's, not the rule's: a body-less declaration —
// func TestStub(t *testing.T) with no braces — parses without error, and
// walking its nil body panics. Such a test mentions nothing.
func mentionedIn(body *ast.BlockStmt) usedNames {
	mentioned := usedNames{}
	if body == nil {
		return mentioned
	}
	ast.Inspect(body, func(node ast.Node) bool {
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
