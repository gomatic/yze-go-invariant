//line no_such_dir_k1n814nr/zz.go:1
// Package linedir pins that the directory this probe reads its test corpus from
// is the one the go tool compiled the package from, and never the one a `//line`
// directive in a source file names.
//
// aim.go carries the directive and sorts first, so it is the file packageDir
// reads. Under a position-derived directory the scan is aimed at a path holding
// no test file at all: testedSymbols reports no tests and run returns before
// reporting anything, so ONE comment line in THIS file silences every claim in
// beta.go and gamma.go too — neither of which carries a marker of any kind, and
// neither of which a reader judging those claims has any reason to open. That is
// a whole-package disablement written in one comment, in one file, and it
// appears in no configuration file and charges no quota.
//
// Scanned is the positive control on the corpus's CONTENT: its claim is silent
// only because the test file in this directory names and reaches it, so a
// silence here would be a silence the directory scan actually earned.
package linedir

// Aimed writes atomically, so no reader observes a partial value.
func Aimed() {} // want "Aimed documents an invariant"

// Scanned is safe to copy: it holds one word.
type Scanned int

// OpenScanned is the constructor whose result type is the mention.
func OpenScanned() Scanned { return 0 }
