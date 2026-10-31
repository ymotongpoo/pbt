// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

// Bounded draws node while the recursion budget of the value being generated
// lasts, and leaf once it is spent. The outermost Bounded draw of a value
// sets the budget to the size parameter; every Bounded draw nested inside
// it, whatever its type, spends one unit. Generators for mutually recursive
// types therefore share one budget, and a value holds about size nodes in
// total. Recursive, Make and the code written by cmd/pbtgen are built on
// it.
//
// Generators are not safe for concurrent draws, and the budget is one more
// reason: it belongs to the test case, not to the goroutine.
func Bounded[V any](leaf, node Gen[V]) Gen[V] {
	return Gen[V]{name: "Bounded(" + leaf.name + ")", draw: func(t *T) V {
		if !t.recActive {
			t.recActive = true
			t.recBudget = max(t.size, 1)
			defer func() { t.recActive = false }()
			return node.drawSpan(t)
		}
		if t.recBudget <= 0 {
			return leaf.drawSpan(t)
		}
		t.recBudget--
		return node.drawSpan(t)
	}}
}

// Recursive builds a generator for a recursive type from a generator of
// leaves and a function that builds the recursive case from a generator of
// sub-values. Each value may create about size sub-values; once that budget
// is spent, every sub-value is a leaf, so the value stays small no matter
// how the recursive case branches. It is the bounded form of the ?LAZY
// recursion of the PropEr Testing book, and the equivalent of Hypothesis's
// recursive strategy.
func Recursive[V any](leaf Gen[V], extend func(self Gen[V]) Gen[V]) Gen[V] {
	var self Gen[V]
	node := Lazy(func() Gen[V] {
		return Frequency(
			Weighted[V]{Weight: 1, Gen: leaf},
			Weighted[V]{Weight: 3, Gen: extend(self)},
		)
	})
	self = Bounded(leaf, node)
	return named(self, "Recursive("+leaf.name+")")
}
