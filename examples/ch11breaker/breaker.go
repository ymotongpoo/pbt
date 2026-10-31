// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ch11breaker ports Chapter 11 of the PropEr Testing book: a circuit
// breaker tested as a finite state machine with named states. The breaker
// mirrors the API of the Klarna library the chapter tests: calls go through
// it, errors and timeouts are counted, enough of them trip it, and it can be
// blocked, deblocked and reset by hand. Time-based recovery is left out, as
// in the chapter's model.
package ch11breaker

import (
	"errors"
	"sync"
)

// Options sets the thresholds: a breaker trips after NErrors errors or
// NTimeouts timeouts.
type Options struct {
	NErrors   int
	NTimeouts int
}

// Breaker guards one service.
type Breaker struct {
	mu       sync.Mutex
	opts     Options
	errors   int
	timeouts int
	tripped  bool
	blocked  bool
}

var (
	// ErrTripped is returned while the breaker is tripped.
	ErrTripped = errors.New("circuit breaker tripped")
	// ErrBlocked is returned while the breaker is blocked by hand.
	ErrBlocked = errors.New("circuit breaker blocked")
	// ErrTimeout is what a call that timed out returns.
	ErrTimeout = errors.New("timeout")
)

// New returns a breaker that is ready to serve.
func New(opts Options) *Breaker { return &Breaker{opts: opts} }

// Call runs f when the breaker allows it. An error returned by f counts
// toward the error threshold, ErrTimeout toward the timeout threshold, and
// a nil result decrements the counters, which is the detail the chapter's
// model got wrong first.
func (b *Breaker) Call(f func() error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.blocked:
		return ErrBlocked
	case b.tripped:
		return ErrTripped
	}
	err := f()
	switch {
	case err == nil:
		b.errors = max(b.errors-1, 0)
		b.timeouts = max(b.timeouts-1, 0)
	case errors.Is(err, ErrTimeout):
		b.timeouts++
	default:
		b.errors++
	}
	if b.errors >= b.opts.NErrors || b.timeouts >= b.opts.NTimeouts {
		b.tripped = true
	}
	return err
}

// Block refuses every call until Deblock.
func (b *Breaker) Block() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocked = true
}

// Deblock lifts a manual block.
func (b *Breaker) Deblock() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocked = false
}

// Reset clears the counters and the tripped state.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.errors, b.timeouts, b.tripped = 0, 0, false
}
