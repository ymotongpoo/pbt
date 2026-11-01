// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"math"
	"testing"
	"unicode/utf8"
)

func TestUint64ReachesEveryHighBitPattern(t *testing.T) {
	// All patterns of bits 52..57 must be reachable with the top bit set.
	// Reusing the width selector as value bits used to allow only pattern 63.
	var seen [64]bool
	g := Uint64()
	rng := newRand(1)
	for range 100_000 {
		c := newT(t.Name(), nil, 42, 42, rng)
		v := g.drawSpan(c)
		if v >= uint64(1)<<63 {
			seen[(v>>52)&63] = true
		}
	}
	for pattern, ok := range seen {
		if !ok {
			t.Errorf("never generated high-bit pattern %06b", pattern)
		}
	}
}

func TestRuneCoversAllUnicodeScalarValues(t *testing.T) {
	// Enumerate the recorded choices, including the maximum, so this checks
	// the domain independently of the sampling distribution.
	g := Rune()
	seen := make([]bool, utf8.MaxRune+1)
	c := newT(t.Name(), nil, 42, 42, nil)
	for choice := uint64(0); choice <= runeTableMax; choice++ {
		c.prefix = []uint64{choice}
		c.choices = c.choices[:0]
		c.metas = c.metas[:0]
		r := g.draw(c)
		if !utf8.ValidRune(r) || seen[r] {
			t.Fatalf("choice %d produced duplicate or invalid rune %U", choice, r)
		}
		seen[r] = true
	}
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if utf8.ValidRune(r) && !seen[r] {
			t.Errorf("unreachable rune %U", r)
		}
	}
	c.prefix = []uint64{math.MaxUint64}
	c.choices = c.choices[:0]
	c.metas = c.metas[:0]
	if r := g.draw(c); r != utf8.MaxRune {
		t.Fatalf("maximum replay choice produced %U", r)
	}
}
