package filename

// Helper writes atomically, so no reader observes a partial value. This file
// sits at the LEFT edge of the "_test.go" literal: the go tool compiles it into
// the package like any other source, because the underscore is part of what the
// suffix means. A matcher widened to "test.go" spares it and nothing else in
// this package notices.
func Helper() {} // want "Helper documents an invariant"
