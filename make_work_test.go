// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"encoding/json"
	"reflect"
	"testing"
)

type stablePlain struct {
	N int
	B bool
	F float64
	A [2]uint8
}
type stableSmall struct {
	S string `pbt:"min=1,max=3"`
	V []int  `pbt:"max=2"`
	P *stablePlain
}
type stableRecursive struct {
	N    int
	Next *stableRecursive
}

// Captured at f59ecff, before the shared work budget. These values use fewer
// than 8*max(size, 1) growth steps and must retain their seeded values.
func TestMakePreservesSmallSeededValues(t *testing.T) {
	cases := []struct {
		seed uint64
		want [3]string
	}{
		{1, [3]string{`{"N":-31,"B":true,"F":-3,"A":[254,1]}`, `{"S":"\f4d","V":[],"P":{"N":10903,"B":false,"F":-1.25,"A":[0,0]}}`, `{"N":-31,"Next":{"N":-543,"Next":{"N":-9223372036854775807,"Next":{"N":-9223372036854775807,"Next":{"N":-1860,"Next":{"N":10903,"Next":null}}}}}}`}},
		{7, [3]string{`{"N":19,"B":true,"F":0,"A":[0,255]}`, "{\"S\":\"\u0082\",\"V\":[],\"P\":{\"N\":-68,\"B\":true,\"F\":8.988465674311584e+307,\"A\":[0,0]}}", `{"N":19,"Next":{"N":19,"Next":null}}`}},
		{42, [3]string{`{"N":149,"B":true,"F":-9007199254740992,"A":[254,0]}`, `{"S":"i","V":[-9223372036854775807],"P":{"N":4,"B":true,"F":48.25,"A":[1,0]}}`, `{"N":149,"Next":{"N":-3,"Next":{"N":-9223372036854775807,"Next":{"N":-1,"Next":null}}}}`}},
	}
	for _, c := range cases {
		for i, v := range []any{Make[stablePlain]().Example(c.seed), Make[stableSmall]().Example(c.seed), Make[stableRecursive]().Example(c.seed)} {
			b, err := json.Marshal(v)
			if err != nil || string(b) != c.want[i] {
				t.Fatalf("seed=%d type=%T got %s want %s: %v", c.seed, v, b, c.want[i], err)
			}
		}
	}
}

type workTree struct {
	Text     string
	Children []workTree
	Next     *workTree
	Values   map[string][]string
}

func workUnits(v reflect.Value) int {
	switch v.Kind() {
	case reflect.String:
		return len([]rune(v.String()))
	case reflect.Pointer:
		if v.IsNil() {
			return 0
		}
		return 1 + workUnits(v.Elem())
	case reflect.Slice, reflect.Array:
		n := 0
		if v.Kind() == reflect.Slice {
			n = v.Len()
		}
		for i := range v.Len() {
			n += workUnits(v.Index(i))
		}
		return n
	case reflect.Map:
		n := v.Len()
		it := v.MapRange()
		for it.Next() {
			n += workUnits(it.Key()) + workUnits(it.Value())
		}
		return n
	case reflect.Struct:
		n := 0
		for i := range v.NumField() {
			n += workUnits(v.Field(i))
		}
		return n
	}
	return 0
}
func checkMakeWork[V any](t *testing.T) {
	t.Helper()
	g := Make[V]()
	for _, size := range []int{0, 1, 10, 100, 1000} {
		for seed := uint64(1); seed <= 20; seed++ {
			pt := newT("work", nil, size, size, newRand(seed))
			v := g.Draw(pt, "")
			limit := 8 * max(size, 1)
			if n := workUnits(reflect.ValueOf(v)); n > limit {
				t.Fatalf("%T size=%d seed=%d work=%d > %d", v, size, seed, n, limit)
			}
			// At most ten retries per map candidate; each node also has fixed
			// scalar/termination choices. This deliberately loose bound is linear.
			if n := len(pt.choices); n > 40*(limit+1) {
				t.Fatalf("%T size=%d seed=%d choices=%d", v, size, seed, n)
			}
		}
	}
}
func TestMakeWorkIsLinear(t *testing.T) {
	checkMakeWork[workTree](t)
	checkMakeWork[[][]string](t)
	checkMakeWork[map[string][]string](t)
	checkMakeWork[[4][][3]string](t)
}
func TestMakeCompletesTagsAndOverridesAfterExhaustion(t *testing.T) {
	type tagged struct {
		Exhaust  string `pbt:"min=10,max=10"`
		Text     string `pbt:"min=3,max=3"`
		N        int    `pbt:"min=7,max=7"`
		Values   []int  `pbt:"min=3,max=3"`
		Next     *node
		Ignored  int `pbt:"-"`
		Override string
	}
	pt := newT("tags", nil, 0, 0, newRand(1))
	v := Make[tagged](WithField[tagged]("Override", StringN(20, 20))).Draw(pt, "")
	if len([]rune(v.Exhaust)) != 10 || len([]rune(v.Text)) != 3 || v.N != 7 || len(v.Values) != 3 || v.Next != nil || v.Ignored != 0 || len([]rune(v.Override)) != 20 {
		t.Fatalf("%+v", v)
	}
	// Exhaustion terminates even a recursive collection with a positive min.
	type required struct {
		Children []required `pbt:"min=2,max=2"`
	}
	pt = newT("recursive minimum", nil, 0, 0, newRand(1))
	r := Make[required]().Draw(pt, "")
	if n := workUnits(reflect.ValueOf(r)); n > 8 {
		t.Fatalf("recursive minimum grew %d nodes", n)
	}
}
func TestMakeShrinksRecursiveWitness(t *testing.T) {
	c := counterexample(t, func(pt *T) {
		v := Make[*node]().Draw(pt, "node")
		if v != nil && v.Next != nil {
			pt.Fatal("two nodes")
		}
	}, Seed(1))
	v := drawn[*node](t, c, 0)
	if v == nil || v.Next == nil || v.Next.Next != nil || v.Value != 0 || v.Next.Value != 0 {
		t.Fatalf("not the minimal two-node witness: %+v", v)
	}
}
func TestMakeBudgetRestoredBetweenDraws(t *testing.T) {
	g := Make[[][]string]()
	a := newT("a", nil, 100, 100, newRand(1))
	g.Draw(a, "")
	start := len(a.choices)
	v := g.Draw(a, "")
	b := newT("b", nil, 100, 100, nil)
	b.prefix = a.choices[start:]
	want := g.Draw(b, "")
	if !reflect.DeepEqual(v, want) || a.makeBudget != nil || b.makeBudget != nil {
		t.Fatal("budget leaked between draws")
	}
}

// Captured at f59ecff with size 0, seed 1. Exactly eight required runes
// consume the whole budget; terminating choices still precede later fields.
func TestMakePreservesValuesExactlyAtBudget(t *testing.T) {
	type exact struct {
		Text  string `pbt:"min=8,max=8"`
		N     int64
		Empty []int `pbt:"max=0"`
	}
	pt := newT("exact", nil, 0, 0, newRand(1))
	v := Make[exact]().Draw(pt, "")
	if v.Text != "\fl\"d\u0083\x14\u0092(" || v.N != -1247650 || v.Empty == nil || len(v.Empty) != 0 || len(pt.choices) != 19 {
		t.Fatalf("value=%+v choices=%d", v, len(pt.choices))
	}
}

// Unexported fields are not generated, so a type that is recursive only
// through them is not recursive for Make, and its required elements are
// completed when the budget is spent.
func TestMakeIgnoresUnexportedRecursion(t *testing.T) {
	type hidden struct {
		X    int
		next *hidden
	}
	type row struct {
		Exhaust string   `pbt:"min=8,max=8"`
		Items   []hidden `pbt:"min=3,max=3"`
	}
	g := Make[row]()
	for seed := range uint64(20) {
		c := newT(t.Name(), nil, 0, 0, newRand(seed))
		if v := g.drawSpan(c); len(v.Items) != 3 {
			t.Fatalf("seed %d: %d items, want 3", seed, len(v.Items))
		}
	}
}
