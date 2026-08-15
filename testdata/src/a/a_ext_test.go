package a_test

import (
	"testing"

	"a"
)

// TestBlind names the unexported blind from the external test package and
// reaches nothing whatever. Writing a package clause that ends _test costs one
// word and acquires no verification, so blind's claim stands.
func TestBlind(t *testing.T) {}

// TestUnseenIsRevealed names the unexported unseen from the same external test
// package, equally unable to spell that name, and drives the exported entry
// whose body reaches it. The `a` here IS the import, and it is the control for
// the test below: the two selections are spelled identically and differ only in
// what the spelling binds.
func TestUnseenIsRevealed(t *testing.T) { a.Reveal() }

// TestEscrowedIsWrittenAtomically names escrowed and writes `a.Hold()`, where
// `a` is a LOCAL declared in this function and spelled like the import. It
// shadows the import only inside this body, so a file-wide answer would break
// TestUnseenIsRevealed above and a spelling-only answer silences this claim.
func TestEscrowedIsWrittenAtomically(t *testing.T) {
	a := holder{}
	a.Hold()
}

// holder is the type whose method the shadowing local really holds.
type holder struct{}

// Hold is holder's method, and it reaches nothing.
func (holder) Hold() {}
