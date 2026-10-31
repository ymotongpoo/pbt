// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"slices"
)

// Any generates values of a few common Go types: int, bool, float64, string,
// []any and map[string]any, nested up to three levels. It stands in for
// PropEr's term() and shrinks toward 0.
func Any() Gen[any] { return named(anyDepth(3), "Any()") }

func anyDepth(depth int) Gen[any] {
	alts := []Weighted[any]{
		{Weight: 3, Gen: Map(Int(), func(x int) any { return x })},
		{Weight: 1, Gen: Map(Bool(), func(x bool) any { return x })},
		{Weight: 1, Gen: Map(Float64(), func(x float64) any { return x })},
		{Weight: 3, Gen: Map(String(), func(x string) any { return x })},
	}
	if depth > 0 {
		inner := Lazy(func() Gen[any] { return anyDepth(depth - 1) })
		alts = append(alts,
			Weighted[any]{Weight: 1, Gen: Map(SliceOf(inner), func(xs []any) any { return xs })},
			Weighted[any]{Weight: 1, Gen: Map(MapOf(String(), inner), func(m map[string]any) any { return m })},
		)
	}
	return Frequency(alts...)
}

// Bytes generates byte slices of any length.
func Bytes() Gen[[]byte] { return named(SliceOf(Byte()), "Bytes()") }

// BytesN generates byte slices with between minLen and maxLen bytes.
func BytesN(minLen, maxLen int) Gen[[]byte] {
	return named(SliceOfN(Byte(), minLen, maxLen), fmt.Sprintf("BytesN(%d, %d)", minLen, maxLen))
}

// Ptr generates pointers to values of elem. With allowNil, one pointer in
// five is nil, and nil is the value pointers shrink to.
func Ptr[E any](elem Gen[E], allowNil bool) Gen[*E] {
	return Gen[*E]{name: fmt.Sprintf("Ptr(%s, %v)", elem.name, allowNil), draw: func(t *T) *E {
		if allowNil && !t.drawBool(0.8) {
			return nil
		}
		v := elem.drawSpan(t)
		return &v
	}}
}

// Permutation generates permutations of values. It shrinks toward the
// original order.
func Permutation[E any](values []E) Gen[[]E] {
	return Gen[[]E]{name: fmt.Sprintf("Permutation(%d values)", len(values)), draw: func(t *T) []E {
		out := slices.Clone(values)
		for i := len(out) - 1; i > 0; i-- {
			j := i - int(t.draw(uint64(i), plainMeta, modeUniform))
			out[i], out[j] = out[j], out[i]
		}
		return out
	}}
}
