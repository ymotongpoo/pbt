// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

type Queue struct {
	next int
	open map[int]bool
}

func NewQueue() *Queue { return &Queue{open: map[int]bool{}} }

func (q *Queue) Issue() int {
	q.next++
	q.open[q.next] = true
	return q.next
}

var errNotOpen = errors.New("ticket is not open")

func (q *Queue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	delete(q.open, id)
	return nil
}

func (q *Queue) Open() int { return len(q.open) }

type ticketQueue interface {
	Issue() int
	Close(id int) error
	Open() int
}

func queueProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var open []pbt.Var

		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(v pbt.Var) { open = append(open, v) },
		})
		closeTicket := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(open), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "close", Args: []any{v},
					Pre:  func() bool { return slices.Contains(open, v) },
					Run:  func(env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
					Next: func(pbt.Var) { open = slices.DeleteFunc(open, func(o pbt.Var) bool { return o == v }) },
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("close failed: %v", got)
						}
						return nil
					},
				}
			})
		}
		count := pbt.Just(pbt.Command{
			Name: "open",
			Run:  func(pbt.Env) any { return q.Open() },
			Post: func(_ pbt.Env, got any) error {
				if got != len(open) {
					return fmt.Errorf("%d open tickets, want %d", got, len(open))
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if len(open) == 0 {
				return pbt.OneOf(issue, count)
			}
			return pbt.Frequency(pbt.W(3, issue), pbt.W(2, closeTicket()), pbt.W(1, count))
		}
		init := func() { open = nil }

		prog := pbt.Commands(init, commands, pbt.ModelState(func() any { return open })).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestQueue(t *testing.T) {
	pbt.Check(t, queueProperty(func() ticketQueue { return NewQueue() }))
}

// dupQueue issues the same number twice in a row: is uniqueness observed indirectly?
type dupQueue struct{ Queue }

func (q *dupQueue) Issue() int {
	if q.next%2 == 1 {
		q.open[q.next] = true
		q.next++ // pretend
		return q.next - 1
	}
	return q.Queue.Issue()
}

func TestQueueDupMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, queueProperty(func() ticketQueue { return &dupQueue{Queue{open: map[int]bool{}}} }), pbt.NoSave(), pbt.NoReplay())
}
