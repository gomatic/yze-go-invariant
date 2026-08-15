package linedir

// Beta writes atomically, so no reader observes a partial value. Nothing in this
// file claims to be anywhere else, and it is reported only because the scan read
// the directory the go tool compiled.
func Beta() {} // want "Beta documents an invariant"
