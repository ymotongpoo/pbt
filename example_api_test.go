// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt_test

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// A property draws values inside its body and reports failures through the
// methods of *pbt.T. Check runs it for the configured number of cases and
// shrinks the first failure.
func ExampleCheck() {
	var t *testing.T // the *testing.T of the test that calls Check
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		sorted := slices.Clone(xs)
		slices.Sort(sorted)
		if len(sorted) != len(xs) {
			t.Fatalf("sorting %v changed the length", xs)
		}
	})
}

// ForAll builds a property from a generator and a body, for the common case
// of one or two inputs.
func ExampleForAll() {
	var t *testing.T
	pbt.Check(t, pbt.ForAll2(pbt.String(), pbt.String(), func(t *pbt.T, a, b string) {
		if !strings.HasPrefix(a+b, a) {
			t.Fatalf("%q is not a prefix of %q", a, a+b)
		}
	}))
}

// Generators compose: Map transforms values, Bind chooses the next generator
// from a value, Filter keeps the values that satisfy a predicate, and
// Frequency weights alternatives. Example samples a generator outside a
// property.
func ExampleGen_Example() {
	even := pbt.Map(pbt.IntRange(0, 50), func(n int) int { return n * 2 })
	pair := pbt.Bind(pbt.IntRange(1, 3), func(n int) pbt.Gen[[]int] { return pbt.SliceOfN(even, n, n) })
	for _, v := range pair.Example(1) {
		if v%2 != 0 {
			fmt.Println("odd value", v)
		}
	}
	fmt.Println("every value is even")
	// Output: every value is even
}

// SampleShrink shows what a generator shrinks toward: the last value of the
// sequence is the simplest one the generator can produce.
func ExampleSampleShrink() {
	var last []int
	for v := range pbt.SampleShrink(pbt.SliceOfN(pbt.IntRange(-100, 100), 2, 5), 7) {
		last = v
	}
	fmt.Println(last)
	// Output: [0 0]
}

// Classify, Collect and Measure describe the distribution of the generated
// data; Check logs it when the property passes. Cover declares the share a
// class must reach, and the CheckCoverage option turns a shortfall into a
// failure.
func ExampleT_Cover() {
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		t.Classify(len(xs) == 0, "empty")
		t.Cover(len(xs) > 10, 20, "long")
		t.Measure("length", float64(len(xs)))
	}, pbt.CheckCoverage())
}

// Make derives a generator from a type. Struct tags bound fields and
// WithField or WithType replace the generator of a field or a type.
func ExampleMake() {
	type Order struct {
		ID    int      `pbt:"min=1"`
		Items []string `pbt:"min=1,max=5"`
		Note  string   `pbt:"-"`
	}
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		o := pbt.Make[Order](pbt.WithField[Order]("Items", pbt.SliceOfN(pbt.StringN(1, 3), 1, 5))).Draw(t, "order")
		if o.ID < 1 || len(o.Items) == 0 || o.Note != "" {
			t.Fatalf("unexpected order %+v", o)
		}
	})
}

// A stateful test generates a sequence of commands against a model, runs
// it against the real system and checks every postcondition. Commands can
// be built inline with Cmd; the model state must be treated as immutable.
func ExampleCommands() {
	type counter struct{ n int }
	type model struct{ n int }
	increment := pbt.Just[pbt.Command[model, *counter]](pbt.Cmd[model, *counter]{
		Name: "increment",
		Run:  func(c *counter, _ pbt.Env) any { c.n++; return c.n },
		Next: func(m model, _ pbt.Var) model { return model{m.n + 1} },
		Post: func(m model, _ pbt.Env, got any) error {
			if got != m.n+1 {
				return fmt.Errorf("got %v, want %d", got, m.n+1)
			}
			return nil
		},
	})
	reset := pbt.Just[pbt.Command[model, *counter]](pbt.Cmd[model, *counter]{
		Name: "reset",
		Run:  func(c *counter, _ pbt.Env) any { c.n = 0; return nil },
		Next: func(model, pbt.Var) model { return model{} },
	})
	commands := func(model) pbt.Gen[pbt.Command[model, *counter]] {
		return pbt.Frequency(
			pbt.Weighted[pbt.Command[model, *counter]]{Weight: 3, Gen: increment},
			pbt.Weighted[pbt.Command[model, *counter]]{Weight: 1, Gen: reset},
		)
	}
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, &counter{})
	})
}

// Var names a result that only exists once a command has run, so that later
// commands can refer to it; Resolve reads it during execution.
func ExampleResolve() {
	type registry struct{ names map[int]string }
	type model struct{ ids []pbt.Var }
	register := pbt.Map(pbt.StringN(1, 5), func(name string) pbt.Command[model, *registry] {
		return pbt.Cmd[model, *registry]{
			Name: "register", Args: []any{name},
			Run:  func(r *registry, _ pbt.Env) any { id := len(r.names) + 1; r.names[id] = name; return id },
			Next: func(m model, v pbt.Var) model { return model{append(slices.Clone(m.ids), v)} },
		}
	})
	lookup := func(m model) pbt.Gen[pbt.Command[model, *registry]] {
		return pbt.Map(pbt.SampledFrom(m.ids), func(v pbt.Var) pbt.Command[model, *registry] {
			return pbt.Cmd[model, *registry]{
				Name: "lookup", Args: []any{v},
				Run: func(r *registry, env pbt.Env) any { return r.names[pbt.Resolve[int](env, v)] },
				Post: func(_ model, _ pbt.Env, got any) error {
					if got == "" {
						return fmt.Errorf("%v was not found", v)
					}
					return nil
				},
			}
		})
	}
	commands := func(m model) pbt.Gen[pbt.Command[model, *registry]] {
		if len(m.ids) == 0 {
			return register
		}
		return pbt.OneOf(register, lookup(m))
	}
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, &registry{names: map[int]string{}})
	})
}

// A parallel program runs its branches concurrently and accepts the result
// when some interleaving of the branches explains it, so every command
// must return something the postcondition can check. The counter guards
// its state with a mutex; a counter that read and wrote without it would
// return duplicate values that no interleaving explains. Run such tests
// under the race detector as well.
func ExampleParallelCommands() {
	type counter struct {
		mu sync.Mutex
		n  int
	}
	type model struct{ n int }
	commands := func(model) pbt.Gen[pbt.Command[model, *counter]] {
		return pbt.Just[pbt.Command[model, *counter]](pbt.Cmd[model, *counter]{
			Name: "increment",
			Run: func(c *counter, _ pbt.Env) any {
				c.mu.Lock()
				defer c.mu.Unlock()
				c.n++
				return c.n
			},
			Next: func(m model, _ pbt.Var) model { return model{m.n + 1} },
			Post: func(m model, _ pbt.Env, got any) error {
				if got != m.n+1 {
					return fmt.Errorf("increment returned %v, want %d", got, m.n+1)
				}
				return nil
			},
		})
	}
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(model{}, commands).Draw(t, "cmds")
		prog.RunParallel(t, &counter{})
	})
}

// Maximize turns a run into a targeted search: after the first case reports
// a utility, Check mutates the best case instead of drawing fresh ones.
func ExampleT_Maximize() {
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		steps := pbt.SliceOf(pbt.SampledFrom([]int{-1, 1})).Draw(t, "steps")
		pos := 0
		for _, s := range steps {
			pos += s
		}
		t.Maximize("distance", float64(max(pos, -pos)))
		if pos > 50 {
			t.Fatalf("the walk reached %d", pos)
		}
	}, pbt.SearchSteps(2000))
}

// CheckExists looks for a case on which the property holds and logs the
// shrunk witness; it fails when none turns up.
func ExampleCheckExists() {
	var t *testing.T
	pbt.CheckExists(t, func(t *pbt.T) {
		x := pbt.IntRange(0, 1000).Draw(t, "x")
		if x*x%97 != 1 {
			t.Fail()
		}
	})
}

// NoPanic calls a function with generated arguments and fails when it
// panics; the arguments come from Make.
func ExampleNoPanic() {
	var t *testing.T
	divide := func(a, b int) int { return a / b }
	pbt.Check(t, pbt.NoPanic(divide))
}
