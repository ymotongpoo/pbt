// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"slices"
	"testing"
)

// The shrinker is deterministic: with the same seed it tries the same
// candidates in the same order, so a saved seed reproduces the whole shrink,
// not only its result.

// siblingGroups draws two interleaved groups of composite siblings with
// different labels, one-element int slices and one-rune strings, under the
// same parent. It fails only while both
// groups are in strictly descending order, so the candidate that sorts either
// group passes and is rejected. The order in which the shrinker tries the two
// sorted candidates is then visible in the stream of evaluated cases.
func siblingGroups(record *[]string) Property {
	return func(t *T) {
		var ints []int
		var strs []string
		for i := 0; i < 4; i++ {
			ints = append(ints, SliceOfN(IntRange(0, 100), 1, 1).Draw(t, "int")[0])
			strs = append(strs, StringOfN(SampledFrom([]rune("abcdefgh")), 1, 1).Draw(t, "str"))
		}
		*record = append(*record, fmt.Sprint(ints, strs))
		for i := 1; i < 4; i++ {
			if ints[i-1] <= ints[i] || strs[i-1] <= strs[i] {
				return
			}
		}
		t.Fatalf("both groups descend: %v %v", ints, strs)
	}
}

func shrinkStream(t *testing.T, seed uint64) ([]string, bool) {
	t.Helper()
	var record []string
	cfg, _ := newConfig([]Option{Seed(seed), Sizes(42, 42), Checks(20000), NoSave()})
	r := &runner{tb: nopTB{}, prop: siblingGroups(&record), cfg: cfg}
	fail, _, err := r.search()
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	if fail == nil {
		return nil, false
	}
	r.shrink(fail)
	return record, true
}

func TestShrinkTriesTheSameCandidatesForTheSameSeed(t *testing.T) {
	found := 0
	for seed := uint64(1); seed <= 200 && found < 5; seed++ {
		first, ok := shrinkStream(t, seed)
		if !ok {
			continue
		}
		found++
		for run := 1; run < 20; run++ {
			again, _ := shrinkStream(t, seed)
			if !slices.Equal(first, again) {
				i := 0
				for i < len(first) && i < len(again) && first[i] == again[i] {
					i++
				}
				t.Fatalf("seed %d, run %d: the candidate streams differ at evaluation %d of %d/%d", seed, run, i, len(first), len(again))
			}
		}
	}
	if found == 0 {
		t.Fatal("no seed found a failure to shrink")
	}
}
