// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch13

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var errState = errors.New("operation not allowed in this state")

// Orders は注文ごとの状態を保持する。
type Orders struct {
	next  int
	state map[int]string
}

func NewOrders() *Orders { return &Orders{state: map[int]string{}} }

func (o *Orders) Create() int {
	o.next++
	o.state[o.next] = "unpaid"
	return o.next
}

func (o *Orders) move(id int, to string, from ...string) error {
	if !slices.Contains(from, o.state[id]) {
		return errState
	}
	o.state[id] = to
	return nil
}

func (o *Orders) Pay(id int) error  { return o.move(id, "paid", "unpaid") }
func (o *Orders) Ship(id int) error { return o.move(id, "shipped", "paid") }

// Cancel は未払いの注文を取り消す。支払い済みの注文は返金済みにする。
func (o *Orders) Cancel(id int) error {
	if o.state[id] == "paid" {
		return o.move(id, "refunded", "paid")
	}
	return o.move(id, "cancelled", "unpaid")
}

func (o *Orders) Reopen(id int) error { return o.move(id, "unpaid", "cancelled") }

func (o *Orders) Status(id int) string { return o.state[id] }

type orderBook interface {
	Create() int
	Pay(id int) error
	Ship(id int) error
	Cancel(id int) error
	Reopen(id int) error
	Status(id int) string
}

// allowed は許される遷移の表である。ここにない組み合わせはすべて拒否される。
var allowed = map[string]map[string]string{
	"pay":    {"unpaid": "paid"},
	"ship":   {"paid": "shipped"},
	"cancel": {"unpaid": "cancelled", "paid": "refunded"},
	"reopen": {"cancelled": "unpaid"},
}

var (
	orderOps    = []string{"pay", "ship", "cancel", "reopen"}
	orderStates = []string{"unpaid", "paid", "shipped", "cancelled", "refunded"}
)

func orderProperty(newOrders func() orderBook) pbt.Property {
	return func(t *pbt.T) {
		o := newOrders()
		do := map[string]func(int) error{
			"pay": o.Pay, "ship": o.Ship, "cancel": o.Cancel, "reopen": o.Reopen,
		}
		byState := map[string][]pbt.Var{} // モデル：状態ごとの注文の集合

		create := pbt.Just(pbt.Command{
			Name: "create",
			Run:  func(pbt.Env) any { return o.Create() },
			Next: func(v pbt.Var) { byState["unpaid"] = append(byState["unpaid"], v) },
		})
		// step は、状態 from の注文へ操作 op を適用するコマンドを作る。
		step := func(op, from string) pbt.CommandGen {
			to := allowed[op][from] // 空文字列なら、拒否されるべき組み合わせである
			return pbt.Map(pbt.SampledFrom(byState[from]), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: op + "@" + from, Args: []any{v},
					Pre: func() bool { return slices.Contains(byState[from], v) },
					Run: func(env pbt.Env) any { return do[op](pbt.Resolve[int](env, v)) },
					Next: func(pbt.Var) {
						if to != "" {
							byState[from] = slices.DeleteFunc(byState[from], func(x pbt.Var) bool { return x == v })
							byState[to] = append(byState[to], v)
						}
					},
					Post: func(_ pbt.Env, got any) error {
						if (to != "") != (got == nil) {
							return fmt.Errorf("%s on %s order returned %v", op, from, got)
						}
						return nil
					},
				}
			})
		}
		// status は、状態 state の注文が本当にその状態にあることを確かめる。
		status := func(state string) pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(byState[state]), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "status@" + state, Args: []any{v},
					Pre: func() bool { return slices.Contains(byState[state], v) },
					Run: func(env pbt.Env) any { return o.Status(pbt.Resolve[int](env, v)) },
					Post: func(_ pbt.Env, got any) error {
						if got != state {
							return fmt.Errorf("status = %v, want %s", got, state)
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			alts := []pbt.Weighted[pbt.Command]{pbt.W(3, create)}
			for _, from := range orderStates {
				if len(byState[from]) == 0 {
					continue
				}
				alts = append(alts, pbt.W(1, status(from)))
				for _, op := range orderOps {
					weight := 1
					if allowed[op][from] != "" {
						weight = 3 // 深い状態へ進むよう、許される遷移を多めに出す
					}
					alts = append(alts, pbt.W(weight, step(op, from)))
				}
			}
			return pbt.Frequency(alts...)
		}
		init := func() { clear(byState) }

		prog := pbt.Commands(init, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

// ---- 変異

// cancelAfterShip は発送後のキャンセルを許す。
type cancelAfterShip struct{ Orders }

func (o *cancelAfterShip) Cancel(id int) error {
	if o.state[id] == "shipped" {
		return o.move(id, "refunded", "shipped")
	}
	return o.Orders.Cancel(id)
}

// reopenPaid は支払い済みの注文を未払いへ戻す。
type reopenPaid struct{ Orders }

func (o *reopenPaid) Reopen(id int) error { return o.move(id, "unpaid", "cancelled", "paid") }

// shipUnpaid は未払いの注文を発送する。
type shipUnpaid struct{ Orders }

func (o *shipUnpaid) Ship(id int) error { return o.move(id, "shipped", "unpaid", "paid") }

// cancelSideEffect はエラーを返すが、発送済みの注文の状態を変えてしまう。
type cancelSideEffect struct{ Orders }

func (o *cancelSideEffect) Cancel(id int) error {
	err := o.Orders.Cancel(id)
	if err != nil && o.state[id] == "shipped" {
		o.state[id] = "refunded"
	}
	return err
}

func TestOrders(t *testing.T) {
	pbt.Check(t, orderProperty(func() orderBook { return NewOrders() }), opts...)
}
func TestCancelAfterShipMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, orderProperty(func() orderBook { return &cancelAfterShip{*NewOrders()} }), opts...)
}
func TestReopenPaidMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, orderProperty(func() orderBook { return &reopenPaid{*NewOrders()} }), opts...)
}
func TestShipUnpaidMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, orderProperty(func() orderBook { return &shipUnpaid{*NewOrders()} }), opts...)
}
func TestCancelSideEffectMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, orderProperty(func() orderBook { return &cancelSideEffect{*NewOrders()} }), opts...)
}

// 到達可能性：各行が実際に通っているかを見る。
func TestOrderCommandMix(t *testing.T) {
	pbt.Check(t, orderProperty(func() orderBook { return NewOrders() }), append(opts, pbt.Verbose())...)
}
