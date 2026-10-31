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

// System is the queue together with the clock the test controls.
type System struct {
	queue *Queue
	clock *clock.Clock
}

func newSystem(opts Options) *System {
	clk := clock.New()
	return &System{queue: New(clk.Now, visibility, maxAttempts, opts), clock: clk}
}

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

type model struct {
	now    time.Duration
	jobs   []jobModel
	leases []leaseModel
}

func (m model) clone() model {
	return model{m.now, slices.Clone(m.jobs), slices.Clone(m.leases)}
}

// settled applies the lease expiries that happened by m.now, the way the
// queue does lazily.
func (m model) settled() model {
	m = m.clone()
	for i := range m.jobs {
		j := &m.jobs[i]
		if j.phase != leased || m.now < j.until {
			continue
		}
		if j.attempts >= maxAttempts {
			j.phase = dead
		} else {
			j.phase, j.readyAt = ready, j.until
		}
	}
	return m
}

// next returns the index of the job a lease takes now, or -1.
func (m model) next() int {
	pick := -1
	for i, j := range m.jobs {
		if j.phase == ready && (pick < 0 || j.readyAt < m.jobs[pick].readyAt) {
			pick = i
		}
	}
	return pick
}

// current reports whether l is the current lease of its job.
func (m model) current(l leaseModel) bool {
	j := m.jobs[l.job]
	return j.phase == leased && j.lease == l.v
}

func (m model) stats() Stats {
	var s Stats
	for _, j := range m.jobs {
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

type cmd = pbt.Cmd[model, *System]
type command = pbt.Command[model, *System]

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

var enqueue = pbt.Map(pbt.SampledFrom(payloads), func(p string) command {
	return cmd{Name: "enqueue", Args: []any{p},
		Run: func(s *System, _ pbt.Env) any { return s.queue.Enqueue(p) },
		Next: func(m model, v pbt.Var) model {
			m = m.clone()
			m.jobs = append(m.jobs, jobModel{id: v, payload: p, readyAt: m.now})
			return m
		}}
})

var lease = pbt.Just[command](cmd{Name: "lease",
	Run: func(s *System, _ pbt.Env) any { l, ok := s.queue.Lease(); return taken{l, ok} },
	Next: func(m model, v pbt.Var) model {
		i := m.next()
		if i < 0 {
			return m
		}
		m = m.clone()
		j := &m.jobs[i]
		j.phase, j.until, j.lease = leased, m.now+visibility, v
		j.attempts++
		m.leases = append(m.leases, leaseModel{v, i})
		return m
	},
	Post: func(m model, env pbt.Env, got any) error {
		r := got.(taken)
		i := m.next()
		switch {
		case i < 0 && r.ok:
			return fmt.Errorf("lease() returned %v, want no job", r.lease)
		case i < 0:
			return nil
		case !r.ok:
			return fmt.Errorf("lease() returned no job, want %v", m.jobs[i].id)
		}
		want := pbt.Resolve[JobID](env, m.jobs[i].id)
		if r.lease.Job != want || r.lease.Payload != m.jobs[i].payload {
			return fmt.Errorf("lease() returned %s %q, want %s %q", r.lease.Job, r.lease.Payload, want, m.jobs[i].payload)
		}
		return nil
	}})

// finish is an ack or a nack of one of the leases issued so far. The lease
// is resolved from the result of the lease command, so the report names
// it by its Var.
func finish(name string, m model, apply func(m model, j *jobModel)) pbt.Gen[command] {
	return pbt.Map(pbt.SampledFrom(m.leases), func(l leaseModel) command {
		return cmd{Name: name, Args: []any{l.v},
			Pre: func(m model) bool { return slices.Contains(m.leases, l) },
			Run: func(s *System, env pbt.Env) any {
				held := pbt.Resolve[taken](env, l.v).lease
				if name == "ack" {
					return s.queue.Ack(held)
				}
				return s.queue.Nack(held)
			},
			Next: func(m model, _ pbt.Var) model {
				if !m.current(l) {
					return m
				}
				m = m.clone()
				apply(m, &m.jobs[l.job])
				return m
			},
			Post: func(m model, _ pbt.Env, got any) error {
				err, _ := got.(error)
				var want error
				if !m.current(l) {
					want = ErrStaleLease
				}
				if !errors.Is(err, want) {
					return fmt.Errorf("%s(%v) returned %v, want %v", name, l.v, err, want)
				}
				return nil
			}}
	})
}

func ack(m model) pbt.Gen[command] {
	return finish("ack", m, func(_ model, j *jobModel) { j.phase = done })
}

func nack(m model) pbt.Gen[command] {
	return finish("nack", m, func(m model, j *jobModel) {
		if j.attempts >= maxAttempts {
			j.phase = dead
		} else {
			j.phase, j.readyAt = ready, m.now
		}
	})
}

var advance = pbt.Map(pbt.SampledFrom(steps), func(d time.Duration) command {
	return cmd{Name: "advance", Args: []any{d.String()},
		Run:  func(s *System, _ pbt.Env) any { s.clock.Advance(d); return nil },
		Next: func(m model, _ pbt.Var) model { m.now += d; return m.settled() }}
})

var stats = pbt.Just[command](cmd{Name: "stats",
	Run: func(s *System, _ pbt.Env) any { return s.queue.Stats() },
	Post: func(m model, _ pbt.Env, got any) error {
		if want := m.stats(); got != want {
			return fmt.Errorf("stats() = %+v, want %+v", got, want)
		}
		return nil
	}})

func commands(m model) pbt.Gen[command] {
	alts := []pbt.Weighted[command]{
		{Weight: 3, Gen: enqueue},
		{Weight: 3, Gen: lease},
		{Weight: 2, Gen: advance},
		{Weight: 1, Gen: stats},
	}
	if len(m.leases) > 0 {
		alts = append(alts, pbt.Weighted[command]{Weight: 2, Gen: ack(m)}, pbt.Weighted[command]{Weight: 1, Gen: nack(m)})
	}
	return pbt.Frequency(alts...)
}

func TestQueue(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, newSystem(Options{}))
	})
}

// The queue that ignores the token accepts an ack from a worker whose
// lease is no longer current while another worker holds the job. The
// shortest program leases the job, gives it back, by a nack or by letting
// the lease expire, leases it again and acks with the first lease.
func TestStaleAckIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, newSystem(Options{IgnoreToken: true}))
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "= lease()") != 2 || !strings.Contains(program, "= ack(var2)") || !strings.Contains(report, "ack(var2) returned <nil>, want jobqueue: lease is not current") {
		t.Errorf("expected two leases and an ack with the first one:\n%s", report)
	}
}
