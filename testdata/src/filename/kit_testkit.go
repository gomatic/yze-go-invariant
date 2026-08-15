package filename

// Kit writes atomically, so no reader observes a partial value. This file sits
// at the RIGHT edge: its name CONTAINS "_test" and does not END in "_test.go",
// so a matcher widened from a suffix to a substring — the shape an author picks
// a filename to satisfy — spares it.
func Kit() {} // want "Kit documents an invariant"
