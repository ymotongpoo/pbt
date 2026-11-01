// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

type JobQueue struct {
	next int
	jobs []int
}

func (q *JobQueue) Enqueue() int { q.next++; q.jobs = append(q.jobs, q.next); return q.next }
func (q *JobQueue) Dequeue() int { j := q.jobs[0]; q.jobs = q.jobs[1:]; return j }
func (q *JobQueue) Len() int     { return len(q.jobs) }
func (q *JobQueue) IDs() []int   { return slices.Clone(q.jobs) }

type jobQueue interface {
	Enqueue() int
	Dequeue() int
	Len() int
	IDs() []int
}

func jobQueueProperty(newQ func() jobQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQ()
		var pending []pbt.Var

		enqueue := pbt.Just(pbt.Command{
			Name: "enqueue",
			Run:  func(pbt.Env) any { return q.Enqueue() },
			Next: func(v pbt.Var) { pending = append(pending, v) },
		})
		dequeue := func() pbt.CommandGen {
			want := pending[0]
			return pbt.Just(pbt.Command{
				Name: "dequeue", Args: []any{want},
				Pre:  func() bool { return len(pending) > 0 && pending[0] == want },
				Run:  func(pbt.Env) any { return q.Dequeue() },
				Next: func(pbt.Var) { pending = pending[1:] },
				Post: func(env pbt.Env, got any) error {
					if w := pbt.Resolve[int](env, want); got != w {
						return fmt.Errorf("dequeue() = %v, want %d", got, w)
					}
					return nil
				},
			})
		}
		length := pbt.Just(pbt.Command{
			Name: "len",
			Run:  func(pbt.Env) any { return q.Len() },
			Post: func(_ pbt.Env, got any) error {
				if got != len(pending) {
					return fmt.Errorf("len() = %v, model says %d", got, len(pending))
				}
				return nil
			},
		})
		audit := pbt.Just(pbt.Command{
			Name: "audit",
			Run:  func(pbt.Env) any { return q.IDs() },
			Post: func(env pbt.Env, got any) error {
				ids := got.([]int)
				if len(ids) != len(pending) {
					return fmt.Errorf("system holds %d jobs, model holds %d", len(ids), len(pending))
				}
				for i, v := range pending {
					if want := pbt.Resolve[int](env, v); ids[i] != want {
						return fmt.Errorf("job %d in system is %d, model says %d", i, ids[i], want)
					}
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if len(pending) == 0 {
				return pbt.Frequency(pbt.W(4, enqueue), pbt.W(1, length), pbt.W(1, audit))
			}
			return pbt.Frequency(pbt.W(4, enqueue), pbt.W(3, dequeue()), pbt.W(1, length), pbt.W(2, audit))
		}
		init := func() { pending = nil }
		prog := pbt.Commands(init, commands, pbt.ModelState(func() any { return pending })).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestJobQueue(t *testing.T) {
	pbt.Check(t, jobQueueProperty(func() jobQueue { return &JobQueue{} }), pbt.Verbose())
}

// mutant: Dequeue forgets to drop the job from the internal slice copy returned by IDs (stale state)
type staleQueue struct{ JobQueue }

func (q *staleQueue) Dequeue() int {
	j := q.jobs[0]
	q.jobs = append([]int{}, q.jobs[1:]...)
	q.jobs = append(q.jobs, j)
	return j
}

func TestJobQueueMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, jobQueueProperty(func() jobQueue { return &staleQueue{} }), pbt.NoSave(), pbt.NoReplay())
}
