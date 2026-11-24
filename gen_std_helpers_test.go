// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"math/rand/v2"
	"testing"
)

// forArbitraryChoices draws g from n prefixes of arbitrary choices, the way
// shrink candidates feed a generator, and calls check on every value. A
// generator built by construction must produce a valid value, or discard,
// for every choice sequence, not only for the ones fresh randomness yields.
func forArbitraryChoices[V any](t *testing.T, g Gen[V], n int, check func(V) string) {
	t.Helper()
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range n {
		prefix := make([]uint64, 1+rng.IntN(40))
		for j := range prefix {
			switch rng.IntN(3) {
			case 0:
				prefix[j] = rng.Uint64()
			case 1:
				prefix[j] = rng.Uint64() >> rng.IntN(64)
			default:
				prefix[j] = uint64(rng.IntN(4))
			}
		}
		c := newT(t.Name(), prefix, 42, 42, nil)
		var v V
		c.execute(func(c *T) { v = g.drawSpan(c) })
		if c.failed {
			t.Fatalf("choices %v: the generator failed: %v %v", prefix, c.panicVal, c.messages)
		}
		if c.discarded {
			continue
		}
		if msg := check(v); msg != "" {
			t.Fatalf("choices %v: %#v: %s", prefix, v, msg)
		}
		_ = i
	}
}

// minimalOf shrinks the first value of g for which fails holds and returns
// it.
func minimalOf[V any](t *testing.T, g Gen[V], fails func(V) bool) V {
	t.Helper()
	c := counterexample(t, func(pt *T) {
		if v := g.Draw(pt, "v"); fails(v) {
			pt.Fatalf("fails")
		}
	})
	return drawn[V](t, c, 0)
}
