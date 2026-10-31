// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: queue
// Queue is the system under test: it issues ticket numbers and closes them.
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

// snippet: end

// snippet: model
// ticketQueue is what the commands need from the system under test.
type ticketQueue interface {
	Issue() int
	Close(id int) error
	Open() int
}

// queueProperty is the stateful property over a queue. The model is what
// the test believes about the queue: the tickets that are open, as Vars,
// because their numbers exist only once Issue has run. The model and the
// queue are variables of the property, and the commands close over them.
func queueProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var open []pbt.Var

		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(v pbt.Var) { open = append(open, v) },
		})
		closeTicket := func() pbt.Gen[pbt.Command] {
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
		// commands chooses the next command from the model: close is only
		// offered while a ticket is open.
		commands := func() pbt.Gen[pbt.Command] {
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

// snippet: end

// snippet: buggy-queue
// leakyQueue forgets to remove a closed ticket, so Open counts it still.
type leakyQueue struct{ Queue }

func (q *leakyQueue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	q.open[id] = false
	return nil
}

// snippet: end

func TestQueueReport(t *testing.T) {
	report := failureReport(t, "TestQueue", queueProperty(func() ticketQueue {
		return &leakyQueue{Queue{open: map[int]bool{}}}
	}), pbt.Seed(1))
	mustContain(t, report, "var1 = issue()", "close(var1)", "open()", "1 open tickets, want 0", "history:", "model state before step 3: []")
}
