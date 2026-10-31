// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package fixture

import (
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
