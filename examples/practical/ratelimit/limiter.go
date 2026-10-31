// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ratelimit is a practical example of a stateful test whose model
// is the specification itself. A limiter allows each client a number of
// requests per sliding window and forgets idle clients to bound its memory.
// The model records the requests it allowed and the time, and the
// postcondition of every request is the rule "at most limit requests in
// any window", which is simpler than the pruned logs the limiter keeps.
package ratelimit

import (
	"sync"
	"time"
)

// Options select the planted bug of the example. The zero value is the
// correct limiter.
type Options struct {
	// EagerSweep makes Sweep forget a client after half a window of
	// inactivity instead of a whole one. A client that is forgotten while
	// its requests are still inside the window gets a fresh quota.
	EagerSweep bool
}

// Limiter allows each client at most limit requests in any window.
type Limiter struct {
	mu     sync.Mutex
	now    func() time.Time
	limit  int
	window time.Duration
	grants map[string][]time.Time // the allowed requests of each client, oldest first
	opts   Options
}

// New returns a limiter. now is the time source.
func New(now func() time.Time, limit int, window time.Duration, opts Options) *Limiter {
	return &Limiter{now: now, limit: limit, window: window, grants: map[string][]time.Time{}, opts: opts}
}

// Allow reports whether client may make a request now and records the
// request when it may.
func (l *Limiter) Allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	log := l.grants[client]
	for len(log) > 0 && now.Sub(log[0]) >= l.window {
		log = log[1:]
	}
	if len(log) >= l.limit {
		l.grants[client] = log
		return false
	}
	l.grants[client] = append(log, now)
	return true
}

// Sweep forgets the clients that made no allowed request during the last
// window and returns how many it forgot. A server calls it periodically.
func (l *Limiter) Sweep() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	idle := l.window
	if l.opts.EagerSweep {
		idle /= 2
	}
	n := 0
	for client, log := range l.grants {
		if now.Sub(log[len(log)-1]) >= idle {
			delete(l.grants, client)
			n++
		}
	}
	return n
}
