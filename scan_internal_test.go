package invariant

import (
	"go/ast"
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

// TestTestedSymbolsReadsBothTestPackages is the point of the directory scan. Go
// splits a package's tests into the internal `package p` files and the external
// `package p_test` ones, which go/analysis presents as separate passes; a pass
// over p can never see p_test. The directory holds both, so a claim verified
// only by an external test still counts as verified.
func TestTestedSymbolsReadsBothTestPackages(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"a.go":           "package a\nfunc Verified() {}\n",
		"a_test.go":      "package a\nfunc TestInternalThing(t *testing.T) {}\nfunc helper() {}\n",
		"a_ext_test.go":  "package a_test\nfunc TestExternalThing(t *testing.T) {}\n",
		"notes.txt":      "not Go at all",
		"broken_test.go": "package a\nthis is not Go\n",
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

	got, hasTests := testedSymbols(dir, read, "/pkg")
	require.True(t, hasTests)

	assert.True(t, isNamedBy("InternalThing", got), "named by the internal test package")
	assert.True(t, isNamedBy("ExternalThing", got), "named by the EXTERNAL test package")
	assert.False(t, isNamedBy("helper", got), "a non-Test function names nothing")
	assert.False(t, isNamedBy("Verified", got), "a source function is not a test name")
}

// TestTestedSymbolsFailsOpen pins that a filesystem failure contributes nothing
// rather than turning every documented claim into a finding.
func TestTestedSymbolsFailsOpen(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	unreadableDir := func(dirPath) ([]string, error) { return nil, errRead }
	names, hasTests := testedSymbols(unreadableDir, os.ReadFile, "/pkg")
	want.Empty(names)
	want.False(hasTests, "an unreadable directory yields no corpus to judge against")

	noTests := func(dirPath) ([]string, error) { return []string{"a.go", "README.md"}, nil }
	_, hasTests = testedSymbols(noTests, os.ReadFile, "/pkg")
	want.False(hasTests, "a package with no tests is the coverage gate's finding, not this probe's")

	oneTest := func(dirPath) ([]string, error) { return []string{"a_test.go"}, nil }
	names, hasTests = testedSymbols(oneTest, func(string) ([]byte, error) { return nil, errRead }, "/pkg")
	want.Empty(names)
	want.True(hasTests, "an unreadable test file still means the package is tested")

	names, _ = testedSymbols(oneTest, func(string) ([]byte, error) { return []byte("package a\n???"), nil }, "/pkg")
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
