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

// thisPackage is the fixture identity: the package under analysis is named `a`
// and sits at the import path `a`, as it does under analysistest.
var thisPackage = packageIdentity{name: "a", path: "a"}

// thisFile is the set of names a file uses for that package when it imports it
// without an alias.
var thisFile = selfNames{"a": true}

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

	got, hasTests := testedSymbols(dir, read, "/pkg", thisPackage, newDeclarations())
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
	pkg := newDeclarations()
	pkg.plain.add("NewConstructed", plainly("Constructed"))

	got, hasTests := testedSymbols(dir, read, "/pkg", thisPackage, pkg)
	require.True(t, hasTests)

	assert.True(t, isNamedBy("Local", got), "reached through a helper in the same test file")
	assert.True(t, isNamedBy("Shared", got), "reached through a helper in the external test package")
	assert.True(t, isNamedBy("Constructed", got), "reached through a constructor in the analyzed package")
	assert.False(t, isNamedBy("Forged", got), "an empty test named for its subject reaches nothing")
}

// TestTestedSymbolsFailsOpen pins that a filesystem failure contributes nothing
// rather than turning every documented claim into a finding.
func TestTestedSymbolsFailsOpen(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	unreadableDir := func(dirPath) ([]string, error) { return nil, errRead }
	names, hasTests := testedSymbols(unreadableDir, os.ReadFile, "/pkg", thisPackage, newDeclarations())
	want.Empty(names)
	want.False(hasTests, "an unreadable directory yields no corpus to judge against")

	noTests := func(dirPath) ([]string, error) { return []string{"a.go", "README.md"}, nil }
	_, hasTests = testedSymbols(noTests, os.ReadFile, "/pkg", thisPackage, newDeclarations())
	want.False(hasTests, "a package with no tests is the coverage gate's finding, not this probe's")

	oneTest := func(dirPath) ([]string, error) { return []string{"a_test.go"}, nil }
	names, hasTests = testedSymbols(
		oneTest,
		func(string) ([]byte, error) { return nil, errRead },
		"/pkg",
		thisPackage,
		newDeclarations(),
	)
	want.Empty(names)
	want.True(hasTests, "an unreadable test file still means the package is tested")

	names, _ = testedSymbols(
		oneTest,
		func(string) ([]byte, error) { return []byte("package a\n???"), nil },
		"/pkg",
		thisPackage,
		newDeclarations(),
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
