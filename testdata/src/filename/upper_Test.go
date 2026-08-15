package filename

// Cased writes atomically, so no reader observes a partial value. The go tool's
// test-file check is CASE SENSITIVE, so this file is ordinary compiled source —
// `go list` reports it in GoFiles — and the claim here must be reported. Nothing
// else in this repo varies the case of a file name, so a matcher folded to lower
// case exempts every `*_Test.go` in the fleet and no other fixture notices.
func Cased() {} // want "Cased documents an invariant"
