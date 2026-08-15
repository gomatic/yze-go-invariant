// Package forgeline pins that whether a declaration sits in a test file is
// decided by the name the FileSet holds — the one the go tool compiled, which no
// directive rewrites — and never by the name a `//line` directive tells the
// position machinery to report.
//
// The three files here carry the same claim and differ only in what they say
// they are. control.go says nothing; forged.go is ordinary compiled source
// claiming a test name, and its claim must still be reported; forgeline_test.go
// is a real test file claiming a source name, and its helper must still be
// spared. Both directions are defects: the first forges a silence, the second
// invents a finding on code that ships no promise to a caller.
package forgeline

// Control writes atomically, so no reader observes a partial value. It is the
// anchor: a silence anywhere else here is a silence a directive bought.
func Control() {} // want "Control documents an invariant"
