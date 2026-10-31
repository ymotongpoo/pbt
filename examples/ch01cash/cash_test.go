// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch01cash

import (
	"errors"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// The unit tests of the chapter, in cents.
func TestCashExamples(t *testing.T) {
	register := []Denomination{{2000, 1}, {1000, 2}, {500, 4}, {100, 10}, {25, 10}, {1, 100}}
	cases := []struct {
		price, paid int
		want        []Denomination
	}{
		{1000, 2000, []Denomination{{1000, 1}}},
		{975, 2000, []Denomination{{1000, 1}, {25, 1}}},
		{82, 100, []Denomination{{1, 18}}},
		{137, 2000, []Denomination{{1000, 1}, {500, 1}, {100, 3}, {25, 2}, {1, 13}}},
	}
	for _, c := range cases {
		got, err := Cash(register, c.price, c.paid)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("Cash(%d, %d) = %v, %v; want %v", c.price, c.paid, got, err, c.want)
		}
	}
	// The counterexample the chapter predicts a framework would find: a
	// register that cannot make the change.
	if _, err := Cash([]Denomination{{500, 1}}, 2000, 3000); !errors.Is(err, ErrCannotMakeChange) {
		t.Errorf("expected ErrCannotMakeChange, got %v", err)
	}
}

var denominations = []int{2000, 1000, 500, 100, 25, 1}

// register generates a register with the usual denominations in descending
// order and a random count of each.
var register = pbt.Custom("register", func(t *pbt.T) []Denomination {
	reg := make([]Denomination, len(denominations))
	for i, v := range denominations {
		reg[i] = Denomination{Value: v, Count: pbt.IntRange(0, 20).Draw(t, "")}
	}
	return reg
})

// The two properties the chapter states: the change adds up to the money
// paid minus the price, and it is handed back from the largest denomination
// to the smallest. The function either returns such change or reports that
// it cannot, instead of crashing.
func TestCashProperties(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		reg := register.Draw(t, "register")
		price := pbt.IntRange(0, 5000).Draw(t, "price")
		paid := pbt.IntRange(price, 10000).Draw(t, "paid")
		change, err := Cash(reg, price, paid)
		if err != nil {
			t.Classify(true, "cannot make change")
			if !errors.Is(err, ErrCannotMakeChange) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		t.Classify(true, "change returned")
		sum := 0
		for i, d := range change {
			sum += d.Value * d.Count
			if i > 0 && change[i-1].Value <= d.Value {
				t.Fatalf("change %v is not ordered from the largest denomination down", change)
			}
			if idx := slices.IndexFunc(reg, func(r Denomination) bool { return r.Value == d.Value }); idx < 0 || reg[idx].Count < d.Count {
				t.Fatalf("change %v uses more %d-cent pieces than the register has", change, d.Value)
			}
		}
		if sum != paid-price {
			t.Fatalf("change %v sums to %d, want %d", change, sum, paid-price)
		}
	})
}
