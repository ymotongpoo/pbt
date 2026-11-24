// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"go.ymotongpoo.com/pbt"
	"testing"
)

// checkMinimumSeeds detects alternatives whose tails fail to realign.
func checkMinimumSeeds(t *testing.T, g pbt.Gen[string], want string) {
	t.Helper()
	for seed := uint64(1); seed <= 10; seed++ {
		var got string
		found := false
		pbt.Check(quietTB{t.Name()}, func(pt *pbt.T) { s := g.Draw(pt, ""); pt.OnFail(func() { got = s; found = true }); pt.Fail() }, pbt.Seed(seed), pbt.Checks(1), pbt.NoSave())
		if !found || got != want {
			t.Errorf("%s seed %d minimum %q, want %q", g, seed, got, want)
		}
	}
}
