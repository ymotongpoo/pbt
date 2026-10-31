// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

// Recursive builds a generator for a recursive type from a generator of
// leaves and a function that builds the recursive case from a generator of
// sub-values. Each top-level draw may create about size sub-values; once
// that budget is spent, every sub-value is a leaf, so the value stays small
// no matter how the recursive case branches. It is the bounded form of the
// ?LAZY recursion of the PropEr Testing book, and the equivalent of
// Hypothesis's recursive strategy.
//
// Generators are not safe for concurrent draws, and the budget is one more
// reason: it belongs to the generator value, not to the test case.
func Recursive[V any](leaf Gen[V], extend func(self Gen[V]) Gen[V]) Gen[V] {
	b := &budget{}
	var node Gen[V]
	self := Gen[V]{name: "Recursive.self", draw: func(t *T) V {
		if b.left <= 0 {
			return leaf.drawSpan(t)
		}
		b.left--
		return node.drawSpan(t)
	}}
	node = Frequency(
		Weighted[V]{Weight: 1, Gen: leaf},
		Weighted[V]{Weight: 3, Gen: extend(self)},
	)
	return Gen[V]{name: "Recursive(" + leaf.name + ")", draw: func(t *T) V {
		saved := b.left
		b.left = max(t.size, 1)
		defer func() { b.left = saved }()
		return node.drawSpan(t)
	}}
}
