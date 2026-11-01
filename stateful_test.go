// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// cache follows Chapter 9 of the PropEr Testing book: a bounded cache that
// overwrites the oldest entry when full and keeps the position of a key that
// is written again.
type cache struct {
	max     int
	entries []cacheEntry
}

type cacheEntry struct{ key, val int }

func (c *cache) find(k int) (int, bool) {
	for _, e := range c.entries {
		if e.key == k {
			return e.val, true
		}
	}
	return 0, false
}

func (c *cache) cache(k, v int) {
	for i := range c.entries {
		if c.entries[i].key == k {
			c.entries[i].val = v
			return
		}
	}
	if len(c.entries) >= c.max {
		c.entries = c.entries[1:]
	}
	c.entries = append(c.entries, cacheEntry{k, v})
}

func (c *cache) flush() { c.entries = nil }

type found struct {
	val int
	ok  bool
}

// cacheProperty tests a cache of realSize against a model of modelSize:
// the same structure, held in variables the commands close over.
func cacheProperty(modelSize, realSize int) Property {
	key := OneOf(IntRange(1, modelSize), Int())
	val := Int()
	return func(t *T) {
		var m, c cache
		commands := func() CommandGen {
			return Frequency(
				W(1, Map(key, func(k int) Command {
					return Command{Name: "find", Args: []any{k},
						Run: func(Env) any { v, ok := c.find(k); return found{v, ok} },
						Post: func(_ Env, res any) error {
							v, ok := m.find(k)
							if res != (found{v, ok}) {
								return fmt.Errorf("find(%d) = %v, model has %v", k, res, found{v, ok})
							}
							return nil
						}}
				})),
				W(3, Custom("cache", func(t *T) Command {
					k, v := key.Draw(t, ""), val.Draw(t, "")
					return Command{Name: "cache", Args: []any{k, v},
						Run:  func(Env) any { c.cache(k, v); return nil },
						Next: func(Var) { m.cache(k, v) }}
				})),
				W(1, Just(Command{Name: "flush",
					Run:  func(Env) any { c.flush(); return nil },
					Next: func(Var) { m.flush() }})),
			)
		}
		init := func() { m = cache{max: modelSize} }
		prog := Commands(init, commands, ModelState(func() any { return m.entries })).Draw(t, "cmds")
		c = cache{max: realSize}
		prog.RunSequential(t)
	}
}

func TestCacheHolds(t *testing.T) {
	tb := &fakeTB{name: "TestCache"}
	Check(tb, cacheProperty(3, 3), Seed(1), NoSave())
	if len(tb.errors) != 0 {
		t.Fatalf("unexpected failure: %q", tb.errors)
	}
	logs := strings.Join(tb.logs, "\n")
	for _, want := range []string{"% cache", "% find", "% flush"} {
		if !strings.Contains(logs, want) {
			t.Errorf("statistics lack %q:\n%s", want, logs)
		}
	}
}

func TestUndersizedCacheIsCaught(t *testing.T) {
	c := counterexample(t, cacheProperty(3, 2))
	prog := drawn[Program](t, c, 0)
	names := make([]string, prog.Len())
	for i, s := range prog.Steps() {
		names[i] = s.Command.Name
	}
	if want := []string{"cache", "cache", "cache", "find"}; !slices.Equal(names, want) {
		t.Errorf("commands %v, want %v\n%s", names, want, prog)
	}
	logs := strings.Join(c.logs, "\n")
	for _, want := range []string{"history:", "1. var1 = cache(", "model state before step 4: ["} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %q:\n%s", want, logs)
		}
	}
	if !strings.Contains(strings.Join(c.messages, "\n"), "postcondition of find(") {
		t.Errorf("messages: %q", c.messages)
	}
}

// registry hands out ids, which the model can only name through Vars.
type registry struct {
	next  int
	names map[int]string
	buggy bool
}

func (r *registry) register(name string) int {
	r.next++
	r.names[r.next] = name
	return r.next
}

func (r *registry) lookup(id int) (string, bool) {
	n, ok := r.names[id]
	return n, ok
}

func (r *registry) unregister(id int) {
	if !r.buggy {
		delete(r.names, id)
	}
}

type regEntry struct {
	name   string
	active bool
}

type found2 struct {
	name string
	ok   bool
}

func registryProperty(buggy bool) Property {
	return func(t *T) {
		known := map[Var]regEntry{}
		var r *registry
		vars := func() []Var {
			vs := slices.Collect(maps.Keys(known))
			slices.SortFunc(vs, func(a, b Var) int { return a.id - b.id })
			return vs
		}
		register := Map(StringN(0, 3), func(name string) Command {
			return Command{Name: "register", Args: []any{name},
				Run:  func(Env) any { return r.register(name) },
				Next: func(v Var) { known[v] = regEntry{name, true} }}
		})
		commands := func() CommandGen {
			vs := vars()
			if len(vs) == 0 {
				return register
			}
			lookup := Map(SampledFrom(vs), func(v Var) Command {
				return Command{Name: "lookup", Args: []any{v},
					Run: func(env Env) any { n, ok := r.lookup(Resolve[int](env, v)); return found2{n, ok} },
					Post: func(_ Env, res any) error {
						e := known[v]
						want := found2{e.name, e.active}
						if !e.active {
							want = found2{}
						}
						if res != want {
							return fmt.Errorf("lookup(%v) = %v, want %v", v, res, want)
						}
						return nil
					}}
			})
			unregister := Map(SampledFrom(vs), func(v Var) Command {
				return Command{Name: "unregister", Args: []any{v},
					Pre:  func() bool { return known[v].active },
					Run:  func(env Env) any { r.unregister(Resolve[int](env, v)); return nil },
					Next: func(Var) { known[v] = regEntry{known[v].name, false} }}
			})
			return Frequency(W(2, register), W(2, lookup), W(1, unregister))
		}
		prog := Commands(func() { clear(known) }, commands).Draw(t, "cmds")
		r = &registry{names: map[int]string{}, buggy: buggy}
		prog.RunSequential(t)
	}
}

func TestRegistryHolds(t *testing.T) {
	Check(t, registryProperty(false), NoSave())
}

func TestRegistryBugIsCaughtThroughVars(t *testing.T) {
	c := counterexample(t, registryProperty(true))
	prog := drawn[Program](t, c, 0)
	names := make([]string, prog.Len())
	for i, s := range prog.Steps() {
		names[i] = s.Command.Name
	}
	if want := []string{"register", "unregister", "lookup"}; !slices.Equal(names, want) {
		t.Errorf("commands %v, want %v\n%s", names, want, prog)
	}
	text := prog.String()
	for _, want := range []string{"var1 = register(\"\")", "var2 = unregister(var1)", "var3 = lookup(var1)"} {
		if !strings.Contains(text, want) {
			t.Errorf("program lacks %q:\n%s", want, text)
		}
	}
}

func TestNoCommandsEndsTheSequence(t *testing.T) {
	Check(t, func(t *T) {
		n := 0
		gen := func() CommandGen {
			if n >= 2 {
				return NoCommands()
			}
			return Just(Command{Name: "step", Next: func(Var) { n++ }})
		}
		prog := Commands(func() { n = 0 }, gen).Draw(t, "cmds")
		if prog.Len() > 2 {
			t.Fatalf("%d commands", prog.Len())
		}
		prog.RunSequential(t)
	}, NoSave())
}

func TestLengthBoundsTheSequence(t *testing.T) {
	step := Just(Command{Name: "step"})
	Check(t, func(t *T) {
		prog := Commands(nil, func() CommandGen { return step }, Length(2, 4)).Draw(t, "cmds")
		if prog.Len() < 2 || prog.Len() > 4 {
			t.Fatalf("%d commands", prog.Len())
		}
	}, NoSave())
}

func TestCommandPanicIsReported(t *testing.T) {
	c := counterexample(t, func(t *T) {
		prog := Commands(nil, func() CommandGen {
			return Map(IntRange(0, 10), func(n int) Command {
				return Command{Name: "boom", Args: []any{n}, Run: func(Env) any {
					if n > 5 {
						panic("too big")
					}
					return nil
				}}
			})
		}, Length(1, -1)).Draw(t, "cmds")
		prog.RunSequential(t)
	})
	if !strings.Contains(strings.Join(c.messages, "\n"), "boom(6) panicked: too big") {
		t.Errorf("messages: %q", c.messages)
	}
}

// The model is replayed from init whenever a sequence is regenerated, so a
// test that forgets to reset a variable in init would see the model of an
// earlier case. This checks that init runs before every generation and
// every execution.
func TestInitRunsBeforeEveryReplay(t *testing.T) {
	Check(t, func(t *T) {
		inits, n := 0, 0
		gen := func() CommandGen { return Just(Command{Name: "step", Next: func(Var) { n++ }}) }
		prog := Commands(func() { inits++; n = 0 }, gen).Draw(t, "cmds")
		if n != prog.Len() {
			t.Fatalf("the model counts %d steps after generating %d", n, prog.Len())
		}
		prog.RunSequential(t)
		if inits != 2 || n != prog.Len() {
			t.Fatalf("init ran %d times and the model counts %d steps after running %d", inits, n, prog.Len())
		}
	}, NoSave())
}

func TestResolveNamesTheWantedType(t *testing.T) {
	env := Env{vals: map[Var]any{{1}: 42, {2}: nil}}
	if got := Resolve[int](env, Var{1}); got != 42 {
		t.Fatalf("got %d", got)
	}
	if got := Resolve[error](env, Var{2}); got != nil {
		t.Fatalf("nil resolved to %v", got)
	}
	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, "holds a int, not a string") {
			t.Errorf("message %q", msg)
		}
	}()
	Resolve[string](env, Var{1})
}
