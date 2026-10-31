// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

// snippet: make
type Order struct {
	ID       int      `pbt:"min=1"`
	Items    []string `pbt:"min=1,max=5"`
	Note     string   `pbt:"-"` // stays zero
	Placed   time.Time
	internal int // unexported fields stay zero
}

var orderGen = pbt.Make[Order](
	pbt.WithType(pbt.Map(pbt.IntRange(0, 364), func(d int) time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d)
	})),
	pbt.WithField[Order]("Items", pbt.SliceOfN(pbt.StringN(1, 8), 1, 5)),
)

func TestOrders(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		o := orderGen.Draw(t, "order")
		if o.ID < 1 || len(o.Items) == 0 || o.Note != "" || o.Placed.Year() != 2026 {
			t.Fatalf("unexpected order %+v", o)
		}
	})
}

// snippet: end

// snippet: recursive
// Expr is a recursive type. Recursive bounds the depth with the size
// parameter, so a value has about size nodes instead of growing without end.
type Expr struct {
	Num  int
	Op   string
	Args []Expr
}

var exprGen = pbt.Recursive(
	pbt.Map(pbt.IntRange(0, 9), func(n int) Expr { return Expr{Num: n} }),
	func(self pbt.Gen[Expr]) pbt.Gen[Expr] {
		return pbt.Custom("op", func(t *pbt.T) Expr {
			return Expr{Op: pbt.SampledFrom([]string{"+", "*"}).Draw(t, ""), Args: pbt.SliceOfN(self, 2, 2).Draw(t, "")}
		})
	},
)

func size(e Expr) int {
	n := 1
	for _, a := range e.Args {
		n += size(a)
	}
	return n
}

func TestExprSize(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		e := exprGen.Draw(t, "expr")
		if size(e) > 4*t.Size()+8 {
			t.Fatalf("%d nodes at size %d", size(e), t.Size())
		}
	})
}

// snippet: end
