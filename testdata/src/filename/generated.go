package filename

// Emitted writes atomically, so no reader observes a partial value. This file
// carries no "Code generated ... DO NOT EDIT." marker and nothing generated it;
// only its NAME suggests otherwise, and a name is free to write. A second
// disjunct keyed on that word is an undeclared, forgeable, file-name exemption
// that adds no statement, so the coverage gate cannot see it and only a fixture
// varying the name can. This kills a disjunct keyed on THIS word and not the
// class: one keyed on another word survives, which is the limit of what a
// fixture pinned to a literal can do.
func Emitted() {} // want "Emitted documents an invariant"
