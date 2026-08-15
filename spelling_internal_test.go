package invariant

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	got := declaredIn(parseSource(t, "a.go", src), thisPackage)

	want.Equal(usedNames{"Reached": true}, got.plain["plain"].plain)
	want.Equal(usedNames{"Reached": true, "in": true, "Input": true, "Output": true}, got.plain["typed"].plain,
		"a signature is where Go keeps type names, so it is part of what the function writes down")
	want.Equal(usedNames{"Selected": true, "s": true, "Store": true}, got.methods["Method"].plain,
		"a method is keyed by its own name, in the table a selection from a value reaches")
	want.NotContains(got.plain, symbolName("Method"), "and never among the package-level declarations")
	want.Empty(got.plain["Bodyless"].plain, "a declaration with nothing written down reaches nothing")
	want.Equal(usedNames{"build": true, "other": true}, got.plain["Table"].plain,
		"a value reaches what its initialiser writes, all of them")
	want.Equal(usedNames{"Make": true}, got.plain["Table"].selected,
		"and a selection from another package is recorded as the member it is")
	want.Equal(got.plain["Table"], got.plain["Second"], "each name in the spec carries the same initialiser")
	want.Equal(usedNames{"Declared": true}, got.plain["Typed"].plain, "an uninitialised value still names its type")
	want.Empty(got.plain["Bare"].plain, "a literal initialiser reaches nothing")
	want.Equal(usedNames{"field": true, "Corner": true}, got.plain["Shape"].plain,
		"a struct declaration is where the types of its fields are spelled")
	want.NotContains(got.plain, symbolName("other"), "an import declares no reachable body")
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
		writtenBy(decls[0].(*ast.FuncDecl), selfNames{"a": true}).plain,
	)
	want.Empty(writtenBy(decls[1].(*ast.FuncDecl), selfNames{"a": true}).plain,
		"a plain declaration with no body writes down nothing")
}

// TestPackageDeclarationsPoolsEveryFileInThePass pins that the analyzed package's own
// bodies arrive as one map, however many files declare them.
func TestPackageDeclarationsPoolsEveryFileInThePass(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	got := packageDeclarations([]*ast.File{
		parseSource(t, "a.go", "package a\nfunc First() { Alpha }\n"),
		parseSource(t, "b.go", "package a\nfunc Second() { Beta }\n"),
	}, thisPackage)

	want.Equal(usedNames{"Alpha": true}, got.plain["First"].plain)
	want.Equal(usedNames{"Beta": true}, got.plain["Second"].plain)
	want.Empty(packageDeclarations(nil, thisPackage).plain, "a pass with no files declares nothing")
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
		"\tsubject.Run(\"case\", func(inner *testing.T) { store.Replace(Direct); a.Reveal() })\n}\n"
	parsed := parseSource(t, "a_test.go", src)

	got := mentionedIn(parsed.Decls[0].(*ast.FuncDecl).Body, thisFile)
	want.True(got.selected["Replace"], "the selected half of store.Replace is a member of store")
	want.False(got.plain["Replace"], "and never a declaration of this package")
	want.True(got.plain["store"], "the qualifier half of store.Replace")
	want.True(got.selected["Run"], "and the selected half of the subtest call, which is a method on a value")
	want.True(got.plain["Reveal"], "while the selected half of a reference qualified by THIS package is a declaration")
	want.True(got.plain["Direct"], "a plain identifier")
	want.True(got.plain["inner"], "an identifier inside a subtest closure")
	want.True(got.selected["T"], "a subtest closure's own signature is inside the body")
	want.False(got.plain["TestThing"], "the test's own name is not something it uses")
	want.False(got.plain["Marker"], "the OUTER signature is outside the body")
	want.False(got.plain["outer"], "the OUTER signature is outside the body")
}

// TestSelfNamesInIsWhatAnIMPORTOfThePackageBinds pins the one discrimination
// the reach walk turns on, in both directions and at its boundary. A qualifier
// naming the package under analysis denotes a declaration of it, and every
// other qualifier is a value or a different package — but ONLY where the file
// imports that package, because Go has no spelling for a package referring to
// itself and a `store` inside `package store` is therefore always a value.
// Deriving the bound name from the last element of the import PATH would be
// wrong: this fleet puts package `authority` at `.../go-authority`.
func TestSelfNamesInIsWhatAnIMPORTOfThePackageBinds(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	const external = "package authority_test\n" +
		"import (\n" +
		"\t\"github.com/gomatic/go-authority\"\n" +
		"\tsame \"github.com/gomatic/go-authority\"\n" +
		"\tother \"github.com/gomatic/go-elsewhere\"\n" +
		"\t\"testing\"\n)\n"
	fleet := packageIdentity{name: "authority", path: "github.com/gomatic/go-authority"}

	got := selfNamesIn(parseSource(t, "a_ext_test.go", external), fleet)

	want.True(got["authority"], "an unaliased import binds the package's own name")
	want.True(got["same"], "and an aliased one binds the alias")
	want.False(got["other"], "an alias bound to a different package is a different package")
	want.False(got["testing"], "and so is an ordinary import")
	want.False(got["go-authority"], "the last element of the path is not what the import binds")

	const internal = "package authority\nimport \"testing\"\n"
	inside := selfNamesIn(parseSource(t, "a_test.go", internal), fleet)

	want.Empty(inside,
		"a file that does not import the package cannot qualify it, so its own name there is a VALUE")
}

// TestImportedPathIsEmptyWhenThePathIsNotAQuotedString pins both directions of
// the import read. The empty result on a malformed spec is the CONTRACT rather
// than a swallowed error: selfNamesIn compares the result to the analyzed
// package's path, and the empty string is not a path any package has, so an
// unreadable spec compares unequal — which is what a failure should do here.
func TestImportedPathIsEmptyWhenThePathIsNotAQuotedString(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.Equal(packagePath("a"), importedPath(&ast.ImportSpec{Path: &ast.BasicLit{Value: `"a"`}}))
	want.Empty(importedPath(&ast.ImportSpec{Path: &ast.BasicLit{Value: "unquoted"}}),
		"an unreadable path names no package, so it matches none")
}
