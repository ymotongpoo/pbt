// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"runtime/debug"
	"time"
)

// Named pairs a sub-property with the name that Conjunction reports.
type Named struct {
	Name string
	Prop Property
}

// Conjunction runs every sub-property on the same test case and fails when
// any of them fails, naming each failed one, as PropEr's conjunction/1 does.
// A FailNow in one sub-property does not stop the others.
func Conjunction(subs ...Named) Property {
	return func(t *T) {
		var failed []string
		for _, sub := range subs {
			if !runSub(t, sub) {
				failed = append(failed, sub.Name)
			}
		}
		if len(failed) > 0 {
			t.Fatalf("conjunction: %v failed", failed)
		}
	}
}

// runSub runs one sub-property, prefixes its messages with its name, and
// reports whether it passed. Discards propagate.
func runSub(t *T, sub Named) (ok bool) {
	t.mu.Lock()
	before, wasFailed := len(t.messages), t.failed
	t.failed = false
	t.mu.Unlock()
	defer func() {
		switch rec := recover().(type) {
		case nil:
		case stopTest:
		case discardTest:
			panic(rec)
		default:
			t.Errorf("panic: %v", rec)
			if t.stack == nil {
				t.stack = debug.Stack()
			}
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		ok = !t.failed
		if !ok && len(t.messages) == before {
			t.messages = append(t.messages, "failed")
		}
		for i := before; i < len(t.messages); i++ {
			t.messages[i] = sub.Name + ": " + t.messages[i]
		}
		t.failed = wasFailed || !ok
	}()
	sub.Prop(t)
	return true
}

// Timeout fails the test case when prop takes longer than d, as PropEr's
// ?TIMEOUT does. Go cannot stop a running function, so the check happens
// after prop returns, the way Hypothesis's deadline setting works; a property
// that never returns is caught by go test's own -timeout instead.
func Timeout(d time.Duration, prop Property) Property {
	return func(t *T) {
		start := time.Now()
		prop(t)
		if elapsed := time.Since(start); elapsed > d {
			t.Fatalf("took %v, longer than the limit of %v", elapsed.Round(time.Millisecond), d)
		}
	}
}

// Recover runs f and returns the value it panicked with, or nil. It lets a
// property treat a panic as a value, for example to assert that invalid input
// panics.
func Recover(f func()) (panicked any) {
	defer func() { panicked = recover() }()
	f()
	return nil
}
