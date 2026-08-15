//line zz_test.go:1
package forgeline

// Forged writes atomically, so no reader observes a partial value. This file is
// compiled, linked and shipped exactly as control.go is — go list reports it in
// GoFiles — and the only thing claiming otherwise is the directive above.
func Forged() {} // want "Forged documents an invariant"
