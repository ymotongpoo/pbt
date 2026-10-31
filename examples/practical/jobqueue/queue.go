// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package jobqueue is a practical example of a stateful test of a queue
// with leases, in the manner of SQS or a database-backed worker queue. A
// worker leases a job, and must acknowledge it before the lease expires;
// an expired or rejected lease returns the job to the queue, and a job
// that used up its attempts goes to the dead letters. The model tracks
// every job and every lease ever issued, so that the test also presents
// leases that expired, and the planted bug is the acknowledgment that
// does not check which lease it is given.
package jobqueue

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// JobID identifies a job.
type JobID string

// Lease is a worker's claim on a job. The token distinguishes the lease
// from earlier ones on the same job.
type Lease struct {
	Job     JobID
	Token   string
	Payload string
}

// Stats counts the jobs in each phase.
type Stats struct {
	Ready, Leased, Done, Dead int
}

// ErrStaleLease is returned when a lease is not the current one of its
// job: it expired, the job was acknowledged already, or it was rejected.
var ErrStaleLease = errors.New("jobqueue: lease is not current")

// Options select the planted bug of the example. The zero value is the
// correct queue.
type Options struct {
	// IgnoreToken makes Ack and Nack check that the job is leased but not
	// that the lease is the current one, so a worker whose lease expired
	// completes a job that another worker holds.
	IgnoreToken bool
}

type phase int

const (
	ready phase = iota
	leased
	done
	dead
)

type job struct {
	id       JobID
	seq      int
	payload  string
	phase    phase
	readyAt  time.Time // when the job became ready, which orders the queue
	until    time.Time // when the current lease expires
	token    string
	attempts int
}

// Queue hands out jobs under leases that expire after the visibility
// period, and gives up on a job after maxAttempts leases.
type Queue struct {
	mu          sync.Mutex
	now         func() time.Time
	visibility  time.Duration
	maxAttempts int
	seq         int
	jobs        map[JobID]*job
	opts        Options
}

// New returns an empty queue. now is the time source.
func New(now func() time.Time, visibility time.Duration, maxAttempts int, opts Options) *Queue {
	return &Queue{now: now, visibility: visibility, maxAttempts: maxAttempts, jobs: map[JobID]*job{}, opts: opts}
}

// Enqueue adds a job and returns its id.
func (q *Queue) Enqueue(payload string) JobID {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.seq++
	id := JobID(fmt.Sprintf("job-%d", q.seq))
	q.jobs[id] = &job{id: id, seq: q.seq, payload: payload, readyAt: q.now()}
	return id
}

// settle returns the jobs whose lease expired to the queue, at the time
// the lease expired, or to the dead letters when they have no attempts
// left.
func (q *Queue) settle(now time.Time) {
	for _, j := range q.jobs {
		if j.phase != leased || now.Before(j.until) {
			continue
		}
		if j.attempts >= q.maxAttempts {
			j.phase = dead
		} else {
			j.phase, j.readyAt = ready, j.until
		}
	}
}

// Lease takes the job that has been ready the longest, with the lowest id
// among ties, and reports false when none is ready.
func (q *Queue) Lease() (Lease, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.now()
	q.settle(now)
	var pick *job
	for _, j := range q.jobs {
		if j.phase != ready {
			continue
		}
		if pick == nil || j.readyAt.Before(pick.readyAt) || j.readyAt.Equal(pick.readyAt) && j.seq < pick.seq {
			pick = j
		}
	}
	if pick == nil {
		return Lease{}, false
	}
	pick.phase = leased
	pick.attempts++
	pick.until = now.Add(q.visibility)
	pick.token = fmt.Sprintf("%s#%d", pick.id, pick.attempts)
	return Lease{Job: pick.id, Token: pick.token, Payload: pick.payload}, true
}

// current returns the job of l when l is its current lease.
func (q *Queue) current(l Lease) (*job, error) {
	j := q.jobs[l.Job]
	if j == nil || j.phase != leased || !q.opts.IgnoreToken && j.token != l.Token {
		return nil, ErrStaleLease
	}
	return j, nil
}

// Ack completes the job of a current lease.
func (q *Queue) Ack(l Lease) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.settle(q.now())
	j, err := q.current(l)
	if err != nil {
		return err
	}
	j.phase = done
	return nil
}

// Nack gives the job of a current lease back to the queue, or to the dead
// letters when it has no attempts left.
func (q *Queue) Nack(l Lease) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.now()
	q.settle(now)
	j, err := q.current(l)
	if err != nil {
		return err
	}
	if j.attempts >= q.maxAttempts {
		j.phase = dead
	} else {
		j.phase, j.readyAt = ready, now
	}
	return nil
}

// Stats counts the jobs in each phase.
func (q *Queue) Stats() Stats {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.settle(q.now())
	var s Stats
	for _, j := range q.jobs {
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
