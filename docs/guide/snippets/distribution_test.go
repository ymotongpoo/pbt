// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"math/bits"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: sized
// A tree whose depth follows the size parameter: small early cases, larger
// later ones. Resize fixes the size of a part of a value.
type Tree struct {
	Value    int
	Children []Tree
}

func tree() pbt.Gen[Tree] {
	return pbt.Sized(func(size int) pbt.Gen[Tree] {
		if size <= 1 {
			return pbt.Map(pbt.Int(), func(v int) Tree { return Tree{Value: v} })
		}
		return pbt.Custom("tree", func(t *pbt.T) Tree {
			child := pbt.Resize(size/2, tree())
			return Tree{Value: pbt.Int().Draw(t, ""), Children: pbt.SliceOfN(child, 0, 3).Draw(t, "")}
		})
	})
}

// snippet: end

func depth(tr Tree) int {
	d := 1
	for _, c := range tr.Children {
		d = max(d, 1+depth(c))
	}
	return d
}

// Halving the size at every level bounds the depth by log2 of the size.
func TestSized(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		tr := tree().Draw(t, "tree")
		if limit := max(bits.Len(uint(t.Size())), 1); depth(tr) > limit {
			t.Fatalf("depth %d at size %d, limit %d", depth(tr), t.Size(), limit)
		}
	})
}

// snippet: frequency
type Op struct {
	Kind string
	Key  string
}

// ops reads four times as often as it writes; shrinking moves toward the
// first alternative, the read.
var ops = pbt.Frequency(
	pbt.W(4, pbt.Map(pbt.StringN(1, 3), func(k string) Op { return Op{"get", k} })),
	pbt.W(1, pbt.Map(pbt.StringN(1, 3), func(k string) Op { return Op{"put", k} })),
)

// snippet: end

// snippet: statistics
func TestOperationMix(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		batch := pbt.SliceOfN(ops, 1, 20).Draw(t, "batch")
		writes := 0
		for _, op := range batch {
			t.Aggregate(op.Kind) // share of each kind among all operations
			if op.Kind == "put" {
				writes++
			}
		}
		t.Classify(writes == 0, "read-only batch") // share of cases
		t.Measure("batch length", float64(len(batch)))
		t.Cover(writes > 0, 60, "has a write")
	}, pbt.CheckCoverage())
}

// snippet: end

func TestStatisticsLogs(t *testing.T) {
	logs := passingLogs(t, "TestOperationMix", func(t *pbt.T) {
		batch := pbt.SliceOfN(ops, 1, 20).Draw(t, "batch")
		writes := 0
		for _, op := range batch {
			t.Aggregate(op.Kind)
			if op.Kind == "put" {
				writes++
			}
		}
		t.Classify(writes == 0, "read-only batch")
		t.Measure("batch length", float64(len(batch)))
		t.Cover(writes > 0, 60, "has a write")
	}, pbt.CheckCoverage(), pbt.Seed(1))
	mustContain(t, logs, "OK, passed", "% get", "% put", "% read-only batch", "batch length: min 1")
}

// snippet: cover-fails
var rareCoverProperty = func(t *pbt.T) {
	n := pbt.IntRange(0, 999).Draw(t, "n")
	t.Cover(n%100 == 0, 30, "round hundred") // too rare for a 30% share
}

// snippet: end

func TestCoverShortfall(t *testing.T) {
	report := failureReport(t, "TestRoundHundreds", rareCoverProperty, pbt.CheckCoverage(), pbt.Seed(1))
	mustContain(t, report, "insufficient coverage", "round hundred:", "required 30.0%")
}
