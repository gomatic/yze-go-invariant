// Package filename varies the one input dimension the test-file matcher is
// keyed on and every other fixture in this repo holds constant: the file's NAME.
//
// Every file elsewhere is either `x.go` or `x_test.go`, so any widening of that
// matcher — either edge of the literal, or a second disjunct beside it — changes
// no verdict anywhere and no case can see it. The four files here sit at
// positions a widened matcher would reach and a correct one does not, so each
// carries the same claim and each must be reported:
//
//   - httptest.go ENDS in "test.go" with no underscore, which is an ordinary,
//     idiomatic, compiled Go filename — net/http/httptest/httptest.go is one.
//     A suffix widened by one character exempts it.
//   - kit_testkit.go CONTAINS "_test" and does not end in "_test.go". A suffix
//     widened to a substring exempts it.
//   - generated.go carries a word an author picks freely and no marker of any
//     kind. A second disjunct keyed on it exempts it, adds no statement, and is
//     therefore invisible to statement coverage.
package filename

// Control writes atomically, so no reader observes a partial value. It is the
// anchor: its name is at no edge, so a silence here is not the matcher's doing.
func Control() {} // want "Control documents an invariant"
