// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"fmt"
	"hash/fnv"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// quietTB runs a Check whose outcome the test does not need.
type quietTB struct{ name string }

func (quietTB) Helper()               {}
func (q quietTB) Name() string        { return q.name }
func (quietTB) Logf(string, ...any)   {}
func (quietTB) Errorf(string, ...any) {}

// checkValid fails when check rejects a value of g, drawn afresh or fed by
// the shrinker. A property that fails for about half of the values makes
// the shrinker try many edited choice sequences, which is how a generator
// meets choices that fresh randomness rarely produces.
func checkValid[V any](t *testing.T, g pbt.Gen[V], check func(V) string) {
	t.Helper()
	pbt.Check(t, func(pt *pbt.T) {
		if msg := check(g.Draw(pt, "v")); msg != "" {
			pt.Fatalf("%s", msg)
		}
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave())
	reported := false
	for seed := uint64(1); seed <= 4; seed++ {
		pbt.Check(quietTB{t.Name()}, func(pt *pbt.T) {
			v := g.Draw(pt, "v")
			if msg := check(v); msg != "" && !reported {
				reported = true
				t.Errorf("while shrinking with seed %d: %#v: %s", seed, v, msg)
			}
			h := fnv.New32a()
			fmt.Fprint(h, v)
			if h.Sum32()%2 == 0 {
				pt.Fail()
			}
		}, pbt.Seed(seed), pbt.NoSave())
	}
}

// minimalOf returns the counterexample Check reports for a property that
// fails when fails holds.
func minimalOf[V any](t *testing.T, g pbt.Gen[V], fails func(V) bool) V {
	t.Helper()
	var got V
	found := false
	pbt.Check(quietTB{t.Name()}, func(pt *pbt.T) {
		v := g.Draw(pt, "v")
		pt.OnFail(func() { got, found = v, true })
		if fails(v) {
			pt.Fail()
		}
	}, pbt.Seed(20261031), pbt.Checks(2000), pbt.NoSave())
	if !found {
		t.Fatalf("%s: no counterexample", g)
	}
	return got
}

// always is a predicate for minimalOf that holds for every value.
func always[V any](V) bool { return true }
