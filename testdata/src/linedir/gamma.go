package linedir

// Gamma writes atomically, so no reader observes a partial value. Beta's
// sibling: two unmarked files rather than one, because a single silenced file
// reads as a per-file exemption and two prove the whole package went dark.
func Gamma() {} // want "Gamma documents an invariant"
