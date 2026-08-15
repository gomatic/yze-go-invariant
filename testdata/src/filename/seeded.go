package filename

// Seeded writes atomically, so no reader observes a partial value. The corpus is
// the directory's TEST files, and this claim is reported because the function
// below is not in one of them.
func Seeded() {} // want "Seeded documents an invariant"

// TestSeededIsWholeBytes is spelled like a test, reaches Seeded, and is compiled
// into the package as ordinary source, which `go test` does not run and no
// coverage run enters. The scan's own test-file filter is the one thing between
// a func spelled Test in a production file and an exemption, so this pins that
// the corpus is seeded from test FILES rather than from anything spelled like a
// test. (This comment describes the fixture and asserts no property, which is
// why it draws no finding of its own.)
func TestSeededIsWholeBytes() { Seeded() }
