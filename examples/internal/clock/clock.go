// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package clock is the fake clock of the examples whose behavior depends on
// time. A command advances it explicitly, so that a lease expires or a
// session times out when the program says so, and the same program gives
// the same results on every run.
package clock

import (
	"sync"
	"time"
)

// Clock moves only when Advance is called.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// New returns a clock set to a fixed instant.
func New() *Clock {
	return &Clock{now: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)}
}

// Now returns the current instant. The method value is what a system
// under test takes as its time source.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d and returns the new instant.
func (c *Clock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now
}
