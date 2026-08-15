// This file holds the claim vocabulary: the words a doc comment uses to ASSERT
// a property rather than to describe behaviour, and the read that finds one.
//
// It is separate from the probe because it is the one thing here that reads
// English rather than Go, and because keeping each file to one thing is what
// lets the 1:1 test-layout rule give each of them its own test file.

package invariant

import "strings"

// claims are the words a doc comment uses to assert a property rather than to
// describe behaviour. Kept deliberately small: each addition widens the probe's
// noise, and the set earns its place by what it finds.
var claims = []string{
	"atomically", "atomic", "never", "always", "exactly", "unambiguous",
	"guarantee", "byte-identical", "byte-exact", "idempotent", "safe to",
	"deduplicat", "must not", "cannot",
}

// claimIn returns the first property-asserting phrase in doc, or empty when the
// documentation only describes.
func claimIn(doc docText) claimText {
	lower := strings.ToLower(string(doc))
	for _, claim := range claims {
		if strings.Contains(lower, claim) {
			return claimText(claim)
		}
	}
	return ""
}
