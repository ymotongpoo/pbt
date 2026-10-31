// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jobqueue

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/clock"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

const (
	visibility  = 30 * time.Second
	maxAttempts = 2
)

var (
	payloads = []string{"resize", "email", "report"}
	// The steps the clock takes: within a lease, exactly a lease, which
	// tests the boundary, and past it.
	steps = []time.Duration{time.Second, 10 * time.Second, visibility, time.Minute}
)

// jobModel is what the test knows about one job. The id is the Var of the
// enqueue that created it, and lease the Var of the lease that holds it.
type jobModel struct {
	id       pbt.Var
	payload  string
	phase    phase
	readyAt  time.Duration
	until    time.Duration
	lease    pbt.Var
	attempts int
}

// leaseModel is a lease that was issued, current or not, and the index of
// its job. Acks and nacks are drawn from these, so that stale leases are
// presented as often as current ones.
type leaseModel struct {
	v   pbt.Var
	job int
}

// harness holds the queue under test, the clock the test controls, and
// the model: the elapsed time, every job and every lease ever issued.
type harness struct {
	queue  *Queue
	clock  *clock.Clock
	now    time.Duration
	jobs   []jobModel
	leases []leaseModel
}

func newHarness(opts Options) *harness {
	clk := clock.New()
	return &harness{queue: New(clk.Now, visibility, maxAttempts, opts), clock: clk}
}

func (h *harness) reset() { h.now, h.jobs, h.leases = 0, nil, nil }

// settle applies the lease expiries that happened by now, the way the
// queue does lazily.
func (h *harness) settle() {
	for i := range h.jobs {
		j := &h.jobs[i]
		if j.phase != leased || h.now < j.until {
			continue
		}
		if j.attempts >= maxAttempts {
			j.phase = dead
		} else {
			j.phase, j.readyAt = ready, j.until
		}
	}
}

// next returns the index of the job a lease takes now, or -1.
func (h *harness) next() int {
	pick := -1
	for i, j := range h.jobs {
		if j.phase == ready && (pick < 0 || j.readyAt < h.jobs[pick].readyAt) {
			pick = i
		}
	}
	return pick
}

// current reports whether l is the current lease of its job.
func (h *harness) current(l leaseModel) bool {
	j := h.jobs[l.job]
	return j.phase == leased && j.lease == l.v
}

func (h *harness) stats() Stats {
	var s Stats
	for _, j := range h.jobs {
		switch j.phase {
		case ready:
			s.Ready++
		case leased:
			s.Leased++
		case done:
			s.Done++
		case dead:
			s.Dead++
		}
	}
	return s
}

// taken is the result of a lease command.
type taken struct {
	lease Lease
	ok    bool
}

// GoString keeps the history readable: the token and the payload, or none.
func (r taken) GoString() string {
	if !r.ok {
		return "none"
	}
	return fmt.Sprintf("%s %q", r.lease.Token, r.lease.Payload)
}

func (h *harness) enqueue() pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(payloads), func(p string) pbt.Command {
		return pbt.Command{Name: "enqueue", Args: []any{p},
			Run:  func(pbt.Env) any { return h.queue.Enqueue(p) },
			Next: func(v pbt.Var) { h.jobs = append(h.jobs, jobModel{id: v, payload: p, readyAt: h.now}) }}
	})
}

func (h *harness) lease() pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "lease",
		Run: func(pbt.Env) any { l, ok := h.queue.Lease(); return taken{l, ok} },
		Next: func(v pbt.Var) {
			i := h.next()
			if i < 0 {
				return
			}
			j := &h.jobs[i]
			j.phase, j.until, j.lease = leased, h.now+visibility, v
			j.attempts++
			h.leases = append(h.leases, leaseModel{v, i})
		},
		Post: func(env pbt.Env, got any) error {
			r := got.(taken)
			i := h.next()
			switch {
			case i < 0 && r.ok:
				return fmt.Errorf("lease() returned %v, want no job", r.lease)
			case i < 0:
				return nil
			case !r.ok:
				return fmt.Errorf("lease() returned no job, want %v", h.jobs[i].id)
			}
			want := pbt.Resolve[JobID](env, h.jobs[i].id)
			if r.lease.Job != want || r.lease.Payload != h.jobs[i].payload {
				return fmt.Errorf("lease() returned %s %q, want %s %q", r.lease.Job, r.lease.Payload, want, h.jobs[i].payload)
			}
			return nil
		}})
}

// finish is an ack or a nack of one of the leases issued so far. The lease
// is resolved from the result of the lease command, so the report names
// it by its Var.
func (h *harness) finish(name string, apply func(j *jobModel)) pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(h.leases), func(l leaseModel) pbt.Command {
		return pbt.Command{Name: name, Args: []any{l.v},
			Pre: func() bool { return slices.Contains(h.leases, l) },
			Run: func(env pbt.Env) any {
				held := pbt.Resolve[taken](env, l.v).lease
				if name == "ack" {
					return h.queue.Ack(held)
				}
				return h.queue.Nack(held)
			},
			Next: func(pbt.Var) {
				if h.current(l) {
					apply(&h.jobs[l.job])
				}
			},
			Post: func(_ pbt.Env, got any) error {
				err, _ := got.(error)
				var want error
				if !h.current(l) {
					want = ErrStaleLease
				}
				if !errors.Is(err, want) {
					return fmt.Errorf("%s(%v) returned %v, want %v", name, l.v, err, want)
				}
				return nil
			}}
	})
}

func (h *harness) ack() pbt.Gen[pbt.Command] {
	return h.finish("ack", func(j *jobModel) { j.phase = done })
}

func (h *harness) nack() pbt.Gen[pbt.Command] {
	return h.finish("nack", func(j *jobModel) {
		if j.attempts >= maxAttempts {
			j.phase = dead
		} else {
			j.phase, j.readyAt = ready, h.now
		}
	})
}

func (h *harness) advance() pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(steps), func(d time.Duration) pbt.Command {
		return pbt.Command{Name: "advance", Args: []any{d.String()},
			Run:  func(pbt.Env) any { h.clock.Advance(d); return nil },
			Next: func(pbt.Var) { h.now += d; h.settle() }}
	})
}

func (h *harness) statsCommand() pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "stats",
		Run: func(pbt.Env) any { return h.queue.Stats() },
		Post: func(_ pbt.Env, got any) error {
			if want := h.stats(); got != want {
				return fmt.Errorf("stats() = %+v, want %+v", got, want)
			}
			return nil
		}})
}

func (h *harness) commands() pbt.Gen[pbt.Command] {
	alts := []pbt.Weighted[pbt.Command]{
		pbt.W(3, h.enqueue()),
		pbt.W(3, h.lease()),
		pbt.W(2, h.advance()),
		pbt.W(1, h.statsCommand()),
	}
	if len(h.leases) > 0 {
		alts = append(alts, pbt.W(2, h.ack()), pbt.W(1, h.nack()))
	}
	return pbt.Frequency(alts...)
}

func (h *harness) program() pbt.Gen[pbt.Program] {
	return pbt.Commands(h.reset, h.commands, pbt.ModelState(func() any { return h.jobs }))
}

func TestQueue(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		newHarness(Options{}).program().Draw(t, "cmds").RunSequential(t)
	})
}

// The queue that ignores the token accepts an ack from a worker whose
// lease is no longer current while another worker holds the job. The
// shortest program leases the job, gives it back, by a nack or by letting
// the lease expire, leases it again and acks with the first lease.
func TestStaleAckIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		newHarness(Options{IgnoreToken: true}).program().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "= lease()") != 2 || !strings.Contains(program, "= ack(var2)") || !strings.Contains(report, "ack(var2) returned <nil>, want jobqueue: lease is not current") {
		t.Errorf("expected two leases and an ack with the first one:\n%s", report)
	}
}
