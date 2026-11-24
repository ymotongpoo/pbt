// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"net/netip"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

// snippet: make
type Order struct {
	ID       int        `pbt:"min=1"`
	Items    []string   `pbt:"min=1,max=5"`
	Note     string     `pbt:"-"` // stays zero
	Placed   time.Time  `pbt:"min=2026-01-01T00:00:00Z,max=2026-12-31T23:59:59Z"`
	Client   netip.Addr // generated with pbt.Addr
	internal int        // unexported fields stay zero
}

var orderGen = pbt.Make[Order](
	pbt.WithField[Order]("Items", pbt.SliceOfN(pbt.StringN(1, 8), 1, 5)),
)

func TestOrders(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		o := orderGen.Draw(t, "order")
		if o.ID < 1 || len(o.Items) == 0 || o.Note != "" || o.Placed.Year() != 2026 || !o.Client.IsValid() {
			t.Fatalf("unexpected order %+v", o)
		}
	})
}

// snippet: end

// snippet: make-recursive
// A directory holds files and directories. Make shares one allowance of
// growth across the whole value, so a tree stays proportional to the size
// parameter however the fields nest.
type Dir struct {
	Name  string
	Files []string
	Dirs  []Dir
}

func countNodes(d Dir) int {
	n := 1 + len(d.Files) + len([]rune(d.Name))
	for _, s := range d.Files {
		n += len([]rune(s))
	}
	for _, c := range d.Dirs {
		n += countNodes(c)
	}
	return n
}

func TestDirectoryTrees(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		d := pbt.Make[Dir]().Draw(t, "dir")
		if n := countNodes(d); n > 8*max(t.Size(), 1)+1 {
			t.Fatalf("%d elements at size %d", n, t.Size())
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
