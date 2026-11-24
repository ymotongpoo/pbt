// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package fixture

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

func depth(tr Tree) int {
	d := 0
	for _, c := range tr.Children {
		d = max(d, 1+depth(c))
	}
	return d
}

func TestGeneratedGenerators(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		o := GenOrder().Draw(t, "order")
		if o.Customer.ID < 1 {
			t.Fatalf("ID %d violates min=1", o.Customer.ID)
		}
		if !strings.HasSuffix(o.Customer.Email, "@example.com") {
			t.Fatalf("Email %q did not come from the tagged generator", o.Customer.Email)
		}
		if o.Customer.Since != (time.Time{}) {
			t.Fatalf("Since was generated although its tag skips it")
		}
		t.Classify(o.Customer.vip, "unexported vip set")
		for _, l := range o.Lines {
			if n := len([]rune(l.SKU)); n < 1 || n > 8 || l.Qty < 1 {
				t.Fatalf("line %+v violates its tags", l)
			}
		}
		if len(o.Notes) > 3 || o.Weight < 0 || o.Weight > 100 {
			t.Fatalf("Notes %d or Weight %v violate their tags", len(o.Notes), o.Weight)
		}
		tr := GenTree().Draw(t, "tree")
		t.Collect(depth(tr))
		c := GenCelsius().Draw(t, "celsius")
		_ = c
	}, pbt.NoSave())
}

// Count every node, including leaves: exhausted budgets must not allocate
// placeholder children that an internal-node count would miss.
func deptNodes(d Dept) int {
	n := 1
	for _, e := range d.Staff {
		n += employeeNodes(e)
	}
	return n
}

func employeeNodes(e Employee) int {
	n := 1
	if e.Dept != nil {
		n += deptNodes(*e.Dept)
	}
	for _, p := range e.Peers {
		n += employeeNodes(p)
	}
	return n
}

// Mutually recursive types share one budget, so all nodes of the
// whole value stay within eight times the size parameter.
func TestMutualRecursionIsBounded(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		d := GenDept().Draw(t, "dept")
		if n := deptNodes(d); n > 8*max(t.Size(), 1)+1 {
			t.Fatalf("%d nodes at size %d", n, t.Size())
		}
		t.Collect(deptNodes(d))
	}, pbt.NoSave())
}

func TestGeneratedStandardTypes(t *testing.T) {
	seen4, seenMapped, seen6, nonzero := false, false, false, false
	pbt.Check(t, func(pt *pbt.T) {
		v := GenStandard().Draw(pt, "v")
		if v.Zone == nil || !v.Addr.IsValid() || !v.Prefix.IsValid() || !v.Port.IsValid() || v.IP.To16() == nil || (len(v.MAC) != 6 && len(v.MAC) != 8) {
			pt.Fatalf("invalid standard value: %+v", v)
		}
		lo, hi := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 123000000, time.UTC)
		if v.Bounded.Before(lo) || v.Bounded.After(hi) || v.MinOnly.Before(lo) || v.MaxOnly.After(hi) {
			pt.Fatalf("time bounds: %+v", v)
		}
		if !v.Override.Equal(time.Unix(123, 0)) {
			pt.Fatal("gen override ignored")
		}
		seen4 = seen4 || len(v.IP) == 4
		seenMapped = seenMapped || (len(v.IP) == 16 && v.IP.To4() != nil)
		seen6 = seen6 || v.IP.To4() == nil
		nonzero = nonzero || !v.Time.IsZero()
	}, pbt.Seed(1), pbt.Checks(500), pbt.NoSave())
	if !seen4 || !seenMapped || !seen6 || !nonzero {
		t.Fatalf("missing forms: %v %v %v %v", seen4, seenMapped, seen6, nonzero)
	}
}

// The generated implementations use the same growth decisions as reflection,
// for both recursive and nonrecursive values, including standard-library types.
func compareMake[V any](t *testing.T, generated pbt.Gen[V], opts ...pbt.MakeOption) {
	t.Helper()
	reflected := pbt.Make[V](opts...)
	for _, size := range []int{0, 1, 10, 100, 1000} {
		var want []V
		pbt.Check(t, func(pt *pbt.T) { want = append(want, reflected.Draw(pt, "")) }, pbt.Seed(1), pbt.Sizes(size, size), pbt.Checks(20), pbt.NoSave(), pbt.NoReplay())
		i := 0
		pbt.Check(t, func(pt *pbt.T) {
			got := generated.Draw(pt, "")
			if !reflect.DeepEqual(got, want[i]) {
				pt.Fatalf("size=%d case=%d: generated %#v != Make %#v", size, i, got, want[i])
			}
			i++
		}, pbt.Seed(1), pbt.Sizes(size, size), pbt.Checks(20), pbt.NoSave(), pbt.NoReplay())
	}
}
func TestGeneratedMatchesMake(t *testing.T) {
	compareMake(t, GenTree())
	compareMake(t, GenDept())
	compareMake(t, GenEmployee())
	compareMake(t, GenCelsius())
	compareMake(t, GenComparable())
	compareMake(t, GenChain())
}
