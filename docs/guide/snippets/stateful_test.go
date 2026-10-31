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
// model is what the test believes about the queue: the tickets that are
// open, as Vars, because their numbers exist only once Issue has run.
type model struct{ open []pbt.Var }

type cmd = pbt.Cmd[model, *Queue]

var issue = pbt.Just[pbt.Command[model, *Queue]](cmd{
	Name: "issue",
	Run:  func(q *Queue, _ pbt.Env) any { return q.Issue() },
	Next: func(m model, v pbt.Var) model { return model{append(slices.Clone(m.open), v)} },
})

func closeTicket(m model) pbt.Gen[pbt.Command[model, *Queue]] {
	return pbt.Map(pbt.SampledFrom(m.open), func(v pbt.Var) pbt.Command[model, *Queue] {
		return cmd{
			Name: "close", Args: []any{v},
			Pre: func(m model) bool { return slices.Contains(m.open, v) },
			Run: func(q *Queue, env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
			Next: func(m model, _ pbt.Var) model {
				return model{slices.DeleteFunc(slices.Clone(m.open), func(o pbt.Var) bool { return o == v })}
			},
			Post: func(_ model, _ pbt.Env, got any) error {
				if got != nil {
					return fmt.Errorf("close failed: %v", got)
				}
				return nil
			},
		}
	})
}

var count = pbt.Just[pbt.Command[model, *Queue]](cmd{
	Name: "open",
	Run:  func(q *Queue, _ pbt.Env) any { return q.Open() },
	Post: func(m model, _ pbt.Env, got any) error {
		if got != len(m.open) {
			return fmt.Errorf("%d open tickets, want %d", got, len(m.open))
		}
		return nil
	},
})

// commands chooses the next command from the model state: close is only
// offered while a ticket is open.
func commands(m model) pbt.Gen[pbt.Command[model, *Queue]] {
	if len(m.open) == 0 {
		return pbt.OneOf(issue, count)
	}
	return pbt.Frequency(
		pbt.Weighted[pbt.Command[model, *Queue]]{Weight: 3, Gen: issue},
		pbt.Weighted[pbt.Command[model, *Queue]]{Weight: 2, Gen: closeTicket(m)},
		pbt.Weighted[pbt.Command[model, *Queue]]{Weight: 1, Gen: count},
	)
}

func TestQueue(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, NewQueue())
	})
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
	type lm = model
	leakyClose := func(m lm) pbt.Gen[pbt.Command[lm, *leakyQueue]] {
		return pbt.Map(pbt.SampledFrom(m.open), func(v pbt.Var) pbt.Command[lm, *leakyQueue] {
			return pbt.Cmd[lm, *leakyQueue]{
				Name: "close", Args: []any{v},
				Pre: func(m lm) bool { return slices.Contains(m.open, v) },
				Run: func(q *leakyQueue, env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
				Next: func(m lm, _ pbt.Var) lm {
					return lm{slices.DeleteFunc(slices.Clone(m.open), func(o pbt.Var) bool { return o == v })}
				},
			}
		})
	}
	issue := pbt.Just[pbt.Command[lm, *leakyQueue]](pbt.Cmd[lm, *leakyQueue]{
		Name: "issue",
		Run:  func(q *leakyQueue, _ pbt.Env) any { return q.Issue() },
		Next: func(m lm, v pbt.Var) lm { return lm{append(slices.Clone(m.open), v)} },
	})
	count := pbt.Just[pbt.Command[lm, *leakyQueue]](pbt.Cmd[lm, *leakyQueue]{
		Name: "open",
		Run:  func(q *leakyQueue, _ pbt.Env) any { return q.Open() },
		Post: func(m lm, _ pbt.Env, got any) error {
			if got != len(m.open) {
				return fmt.Errorf("%d open tickets, want %d", got, len(m.open))
			}
			return nil
		},
	})
	gen := func(m lm) pbt.Gen[pbt.Command[lm, *leakyQueue]] {
		if len(m.open) == 0 {
			return pbt.OneOf(issue, count)
		}
		return pbt.OneOf(issue, leakyClose(m), count)
	}
	report := failureReport(t, "TestQueue", func(t *pbt.T) {
		prog := pbt.Commands(lm{}, gen).Draw(t, "cmds")
		prog.RunSequential(t, &leakyQueue{Queue{open: map[int]bool{}}})
	}, pbt.Seed(1))
	mustContain(t, report, "var1 = issue()", "close(var1)", "open()", "1 open tickets, want 0", "history:")
}
