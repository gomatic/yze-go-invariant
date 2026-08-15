package invariant

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	errs "github.com/gomatic/go-error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errRead is the failure an injected reader returns to exercise fail-open paths.
const errRead errs.Const = "cannot read"

// parseSource parses one source file for the tests that need an *ast.File.
func parseSource(t *testing.T, name, src string) *ast.File {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	require.NoError(t, err)
	return parsed
}

// TestTestedSymbolsReadsBothTestPackages is the point of the directory scan. Go
// splits a package's tests into the internal `package p` files and the external
// `package p_test` ones, which go/analysis presents as separate passes; a pass
// over p can never see p_test. The directory holds both, so a claim verified
// only by an external test still counts as verified.
func TestTestedSymbolsReadsBothTestPackages(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"a.go":           "package a\nfunc Verified() {}\n",
		"a_test.go":      "package a\nfunc TestInternalThing(t *testing.T) { InternalThing() }\nfunc helper() { helper() }\n",
		"a_ext_test.go":  "package a_test\nfunc TestExternalThing(t *testing.T) { a.ExternalThing() }\n",
		"stub_test.go":   "package a\nfunc TestStub(t *testing.T)\n",
		"notes.txt":      "not Go at all",
		"broken_test.go": "package a\nthis is not Go\n",
		"bench_test.go": "package a\n" +
			"func BenchmarkBenched(b *testing.B) { Benched() }\n" +
			"func ExampleExampled()              { Exampled() }\n" +
			"func FuzzFuzzed(f *testing.F)       { Fuzzed() }\n",
	}
	dir := func(dirPath) ([]string, error) {
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		return names, nil
	}
	read := func(path string) ([]byte, error) {
		src, ok := files[filepath.Base(path)]
		if !ok {
			return nil, errRead
		}
		return []byte(src), nil
	}

	got, hasTests := testedSymbols(dir, read, "/pkg", nil)
	require.True(t, hasTests)

	assert.True(t, isNamedBy("InternalThing", got), "named and reached by the internal test package")
	assert.True(t, isNamedBy("ExternalThing", got), "named by the EXTERNAL test package, reached through the qualifier")
	assert.False(t, isNamedBy("helper", got), "a non-Test function names nothing, however much it reaches")
	assert.False(t, isNamedBy("Verified", got), "a source function is not a test name")
	assert.False(t, isNamedBy("Stub", got), "a body-less test reaches nothing, and walking it must not panic")
	assert.False(t, isNamedBy("Benched", got), "a Benchmark exercises its subject under go test and is not a test here")
	assert.False(t, isNamedBy("Exampled", got), "and neither is an Example")
	assert.False(t, isNamedBy("Fuzzed", got), "and neither is a Fuzz target")
}

// TestTestedSymbolsReachesThroughBothSetsOfBodies pins the reach the corpus is
// built with: the naming test's subject counts when it is spelled in a helper
// declared beside the test, in the OTHER test package, or in the package under
// analysis — and the empty test named for its subject still counts for nothing.
func TestTestedSymbolsReachesThroughBothSetsOfBodies(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"a_test.go": "package a\n" +
			"func TestLocal(t *testing.T) { drive() }\n" +
			"func drive() { Local() }\n" +
			"func TestConstructed(t *testing.T) { NewConstructed() }\n" +
			"func TestForged(t *testing.T) {}\n",
		"a_ext_test.go": "package a_test\nfunc TestShared(t *testing.T) { shared() }\nfunc shared() { a.Shared() }\n",
	}
	dir := func(dirPath) ([]string, error) { return []string{"a_test.go", "a_ext_test.go"}, nil }
	read := func(path string) ([]byte, error) { return []byte(files[filepath.Base(path)]), nil }
	pkg := declarations{"NewConstructed": usedNames{"Constructed": true}}

	got, hasTests := testedSymbols(dir, read, "/pkg", pkg)
	require.True(t, hasTests)

	assert.True(t, isNamedBy("Local", got), "reached through a helper in the same test file")
	assert.True(t, isNamedBy("Shared", got), "reached through a helper in the external test package")
	assert.True(t, isNamedBy("Constructed", got), "reached through a constructor in the analyzed package")
	assert.False(t, isNamedBy("Forged", got), "an empty test named for its subject reaches nothing")
}

// TestDeclaredInReadsEveryBodyAndInitialiser pins what the reach walk expands
// through: every function and method a file declares, keyed by its own name,
// and every var or const initialiser, keyed by the name it initialises. The
// second is not decoration — this fleet builds its analyzers as
// `var Analyzer = newAnalyzer()`, so a walk that stopped at functions would
// stop one hop short of everything a test reaches by driving one.
func TestDeclaredInReadsEveryBodyAndInitialiser(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	const src = "package a\n" +
		"import \"other\"\n" +
		"var Table, Second = build(), other.Make()\n" +
		"var Typed Declared\n" +
		"const Bare = 1\n" +
		"type Shape struct{ field Corner }\n" +
		"func plain() { Reached }\n" +
		"func typed(in Input) Output { Reached }\n" +
		"func (s Store) Method() { Selected }\n" +
		"func Bodyless()\n"
	got := declaredIn(parseSource(t, "a.go", src))

	want.Equal(usedNames{"Reached": true}, got["plain"])
	want.Equal(usedNames{"Reached": true, "in": true, "Input": true, "Output": true}, got["typed"],
		"a signature is where Go keeps type names, so it is part of what the function writes down")
	want.Equal(usedNames{"Selected": true, "s": true, "Store": true}, got["Method"],
		"a method is keyed by its own name, and its receiver type is one of its names")
	want.Empty(got["Bodyless"], "a declaration with nothing written down reaches nothing")
	want.Equal(usedNames{"build": true, "other": true, "Make": true}, got["Table"],
		"a value reaches what its initialiser writes, all of them")
	want.Equal(got["Table"], got["Second"], "each name in the spec carries the same initialiser")
	want.Equal(usedNames{"Declared": true}, got["Typed"], "an uninitialised value still names its type")
	want.Empty(got["Bare"], "a literal initialiser reaches nothing")
	want.Equal(usedNames{"field": true, "Corner": true}, got["Shape"],
		"a struct declaration is where the types of its fields are spelled")
	want.NotContains(got, symbolName("other"), "an import declares no reachable body")
}

// TestWrittenByReadsBodySignatureAndReceiver pins what ONE function
// declaration writes down, which is the whole of what a test reaches by naming
// it. The signature and the receiver are in there deliberately: a Go type
// identifier lives in declarations, so a walk over bodies alone could not reach
// a type at all.
func TestWrittenByReadsBodySignatureAndReceiver(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	const src = "package a\nfunc (s Store) Method(in Input) Output { Selected }\nfunc Bare()\n"
	decls := parseSource(t, "a.go", src).Decls

	want.Equal(
		usedNames{"Selected": true, "in": true, "Input": true, "Output": true, "s": true, "Store": true},
		writtenBy(decls[0].(*ast.FuncDecl)),
	)
	want.Empty(writtenBy(decls[1].(*ast.FuncDecl)), "a plain declaration with no body writes down nothing")
}

// TestPackageDeclarationsPoolsEveryFileInThePass pins that the analyzed package's own
// bodies arrive as one map, however many files declare them.
func TestPackageDeclarationsPoolsEveryFileInThePass(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	got := packageDeclarations([]*ast.File{
		parseSource(t, "a.go", "package a\nfunc First() { Alpha }\n"),
		parseSource(t, "b.go", "package a\nfunc Second() { Beta }\n"),
	})

	want.Equal(usedNames{"Alpha": true}, got["First"])
	want.Equal(usedNames{"Beta": true}, got["Second"])
	want.Empty(packageDeclarations(nil), "a pass with no files declares nothing")
}

// TestMentionedInReadsEveryIdentifierInTheBody pins what counts as a test USING
// a symbol: any identifier the body writes, either half of a qualified
// reference, and everything inside a subtest closure including that closure's
// own signature. The boundary is the OUTER declaration — its name and its
// parameter types are not things the test uses.
func TestMentionedInReadsEveryIdentifierInTheBody(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	const src = "package a\n" +
		"func TestThing(subject *outer.Marker) {\n" +
		"\tsubject.Run(\"case\", func(inner *testing.T) { store.Replace(Direct) })\n}\n"
	parsed := parseSource(t, "a_test.go", src)

	got := mentionedIn(parsed.Decls[0].(*ast.FuncDecl).Body)
	want.True(got["Replace"], "the selected half of store.Replace")
	want.True(got["store"], "the qualifier half of store.Replace")
	want.True(got["Direct"], "a plain identifier")
	want.True(got["inner"], "an identifier inside a subtest closure")
	want.True(got["T"], "a subtest closure's own signature is inside the body")
	want.False(got["TestThing"], "the test's own name is not something it uses")
	want.False(got["Marker"], "the OUTER signature is outside the body")
	want.False(got["outer"], "the OUTER signature is outside the body")
}

// TestTestedSymbolsFailsOpen pins that a filesystem failure contributes nothing
// rather than turning every documented claim into a finding.
func TestTestedSymbolsFailsOpen(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	unreadableDir := func(dirPath) ([]string, error) { return nil, errRead }
	names, hasTests := testedSymbols(unreadableDir, os.ReadFile, "/pkg", nil)
	want.Empty(names)
	want.False(hasTests, "an unreadable directory yields no corpus to judge against")

	noTests := func(dirPath) ([]string, error) { return []string{"a.go", "README.md"}, nil }
	_, hasTests = testedSymbols(noTests, os.ReadFile, "/pkg", nil)
	want.False(hasTests, "a package with no tests is the coverage gate's finding, not this probe's")

	oneTest := func(dirPath) ([]string, error) { return []string{"a_test.go"}, nil }
	names, hasTests = testedSymbols(oneTest, func(string) ([]byte, error) { return nil, errRead }, "/pkg", nil)
	want.Empty(names)
	want.True(hasTests, "an unreadable test file still means the package is tested")

	names, _ = testedSymbols(
		oneTest,
		func(string) ([]byte, error) { return []byte("package a\n???"), nil },
		"/pkg",
		nil,
	)
	want.Empty(names)
}

// TestOsReadDirNamesListsEntriesOrFails pins the real reader both ways.
func TestOsReadDirNamesListsEntriesOrFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package a"), 0o600))

	names, err := osReadDirNames(dirPath(dir))
	require.NoError(t, err)
	assert.Equal(t, []string{"a_test.go"}, names)

	_, err = osReadDirNames(dirPath(filepath.Join(dir, "absent")))
	assert.Error(t, err, "an unreadable directory is an error, not an empty listing")
}

// TestPackageDirIsTheDirectoryOfTheFirstFile pins the directory lookup and the
// no-files case.
func TestPackageDirIsTheDirectoryOfTheFirstFile(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	added := fset.AddFile("/src/pkg/a.go", -1, 20)

	assert.Equal(t, dirPath("/src/pkg"), packageDir(fset, []*ast.File{{Package: added.Pos(0)}}))
	assert.Empty(t, packageDir(fset, nil), "no files means no directory")
}
