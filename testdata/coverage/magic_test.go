// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package coverage_test

import (
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/testdata/coverage"
)

func TestMagicRandom(t *testing.T) { checkMagic(t, false) }
func TestMagicGuided(t *testing.T) { checkMagic(t, true) }

func checkMagic(t *testing.T, guided bool) {
	t.Helper()
	if testing.CoverMode() == "" {
		t.Fatal("fixture must run with coverage")
	}
	const budget = 100000
	opts := []pbt.Option{pbt.Seed(1), pbt.Checks(budget), pbt.Sizes(42, 42), pbt.NoSave(), pbt.NoReplay()}
	if guided {
		opts = append(opts, pbt.CoverageGuided())
	}
	firstHit, attempts := 0, 0
	pbt.Check(t, func(c *pbt.T) {
		var b [6]byte
		for i := range b {
			b[i] = pbt.ByteRange(0, 15).Draw(c, "byte")
		}
		attempts++
		if coverage.Magic(b) && firstHit == 0 {
			firstHit = attempts
		}
	}, opts...)
	if attempts != budget || (firstHit > 0) != guided {
		t.Fatalf("guided=%t: first hit %d after %d cases (budget %d)", guided, firstHit, attempts, budget)
	}
	t.Logf("guided=%t: first hit %d, cases %d", guided, firstHit, attempts)
}
