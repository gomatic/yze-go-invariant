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
// whose body reaches it.
func TestUnseenIsRevealed(t *testing.T) { a.Reveal() }
