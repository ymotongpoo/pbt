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

// cacheModel is the model: the same structure, kept immutable.
type cacheModel struct {
	max     int
	entries []cacheEntry
}

func (m cacheModel) find(k int) (int, bool) {
	for _, e := range m.entries {
		if e.key == k {
			return e.val, true
		}
	}
	return 0, false
}

func (m cacheModel) cache(k, v int) cacheModel {
	entries := slices.Clone(m.entries)
	for i := range entries {
		if entries[i].key == k {
			entries[i].val = v
			return cacheModel{m.max, entries}
		}
	}
	if len(entries) >= m.max {
		entries = entries[1:]
	}
	return cacheModel{m.max, append(entries, cacheEntry{k, v})}
}

type cacheCmd = Cmd[cacheModel, *cache]

type found struct {
	val int
	ok  bool
}

func cacheCommands(size int) func(cacheModel) Gen[Command[cacheModel, *cache]] {
	key := OneOf(IntRange(1, size), Int())
	val := Int()
	return func(cacheModel) Gen[Command[cacheModel, *cache]] {
		return Frequency(
			Weighted[Command[cacheModel, *cache]]{Weight: 1, Gen: Map(key, func(k int) Command[cacheModel, *cache] {
				return cacheCmd{Name: "find", Args: []any{k},
					Run: func(c *cache, _ Env) any { v, ok := c.find(k); return found{v, ok} },
					Post: func(m cacheModel, _ Env, res any) error {
						v, ok := m.find(k)
						if res != (found{v, ok}) {
							return fmt.Errorf("find(%d) = %v, model has %v", k, res, found{v, ok})
						}
						return nil
					}}
			})},
			Weighted[Command[cacheModel, *cache]]{Weight: 3, Gen: Custom("cache", func(t *T) Command[cacheModel, *cache] {
				k, v := key.Draw(t, ""), val.Draw(t, "")
				return cacheCmd{Name: "cache", Args: []any{k, v},
					Run:  func(c *cache, _ Env) any { c.cache(k, v); return nil },
					Next: func(m cacheModel, _ Var) cacheModel { return m.cache(k, v) }}
			})},
			Weighted[Command[cacheModel, *cache]]{Weight: 1, Gen: Just[Command[cacheModel, *cache]](cacheCmd{Name: "flush",
				Run:  func(c *cache, _ Env) any { c.flush(); return nil },
				Next: func(m cacheModel, _ Var) cacheModel { return cacheModel{max: m.max} }})},
		)
	}
}

func cacheProperty(modelSize, realSize int) Property {
	return func(t *T) {
		prog := Commands(cacheModel{max: modelSize}, cacheCommands(modelSize)).Draw(t, "cmds")
		prog.RunSequential(t, &cache{max: realSize})
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
	prog := drawn[Program[cacheModel, *cache]](t, c, 0)
	names := make([]string, prog.Len())
	for i, s := range prog.Steps() {
		names[i] = CommandName(s.Command)
	}
	if want := []string{"cache", "cache", "cache", "find"}; !slices.Equal(names, want) {
		t.Errorf("commands %v, want %v\n%s", names, want, prog)
	}
	logs := strings.Join(c.logs, "\n")
	if !strings.Contains(logs, "history:") || !strings.Contains(logs, "1. var1 = cache(") {
		t.Errorf("history missing from logs:\n%s", logs)
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

type regModel struct{ known map[Var]regEntry }

func (m regModel) vars() []Var {
	vs := slices.Collect(maps.Keys(m.known))
	slices.SortFunc(vs, func(a, b Var) int { return a.id - b.id })
	return vs
}

func (m regModel) with(v Var, e regEntry) regModel {
	known := maps.Clone(m.known)
	if known == nil {
		known = map[Var]regEntry{}
	}
	known[v] = e
	return regModel{known}
}

type regCmd = Cmd[regModel, *registry]

func registryCommands(m regModel) Gen[Command[regModel, *registry]] {
	register := Map(StringN(0, 3), func(name string) Command[regModel, *registry] {
		return regCmd{Name: "register", Args: []any{name},
			Run:  func(r *registry, _ Env) any { return r.register(name) },
			Next: func(m regModel, v Var) regModel { return m.with(v, regEntry{name, true}) }}
	})
	vars := m.vars()
	if len(vars) == 0 {
		return register
	}
	lookup := Map(SampledFrom(vars), func(v Var) Command[regModel, *registry] {
		return regCmd{Name: "lookup", Args: []any{v},
			Run: func(r *registry, env Env) any { n, ok := r.lookup(Resolve[int](env, v)); return found2{n, ok} },
			Post: func(m regModel, _ Env, res any) error {
				e := m.known[v]
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
	unregister := Map(SampledFrom(vars), func(v Var) Command[regModel, *registry] {
		return regCmd{Name: "unregister", Args: []any{v},
			Pre:  func(m regModel) bool { return m.known[v].active },
			Run:  func(r *registry, env Env) any { r.unregister(Resolve[int](env, v)); return nil },
			Next: func(m regModel, _ Var) regModel { return m.with(v, regEntry{m.known[v].name, false}) }}
	})
	return Frequency(
		Weighted[Command[regModel, *registry]]{Weight: 2, Gen: register},
		Weighted[Command[regModel, *registry]]{Weight: 2, Gen: lookup},
		Weighted[Command[regModel, *registry]]{Weight: 1, Gen: unregister},
	)
}

type found2 struct {
	name string
	ok   bool
}

func registryProperty(buggy bool) Property {
	return func(t *T) {
		prog := Commands(regModel{}, registryCommands).Draw(t, "cmds")
		prog.RunSequential(t, &registry{names: map[int]string{}, buggy: buggy})
	}
}

func TestRegistryHolds(t *testing.T) {
	Check(t, registryProperty(false), NoSave())
}

func TestRegistryBugIsCaughtThroughVars(t *testing.T) {
	c := counterexample(t, registryProperty(true))
	prog := drawn[Program[regModel, *registry]](t, c, 0)
	names := make([]string, prog.Len())
	for i, s := range prog.Steps() {
		names[i] = CommandName(s.Command)
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
	type sys struct{}
	gen := func(n int) Gen[Command[int, sys]] {
		if n >= 2 {
			return NoCommands[int, sys]()
		}
		return Just[Command[int, sys]](Cmd[int, sys]{Name: "step", Next: func(n int, _ Var) int { return n + 1 }})
	}
	Check(t, func(t *T) {
		prog := Commands(0, gen).Draw(t, "cmds")
		if prog.Len() > 2 {
			t.Fatalf("%d commands", prog.Len())
		}
		prog.RunSequential(t, sys{})
	}, NoSave())
}

func TestCommandPanicIsReported(t *testing.T) {
	type sys struct{}
	c := counterexample(t, func(t *T) {
		prog := CommandsN(0, func(int) Gen[Command[int, sys]] {
			return Map(IntRange(0, 10), func(n int) Command[int, sys] {
				return Cmd[int, sys]{Name: "boom", Args: []any{n}, Run: func(sys, Env) any {
					if n > 5 {
						panic("too big")
					}
					return nil
				}}
			})
		}, 1, -1).Draw(t, "cmds")
		prog.RunSequential(t, sys{})
	})
	if !strings.Contains(strings.Join(c.messages, "\n"), "boom(6) panicked: too big") {
		t.Errorf("messages: %q", c.messages)
	}
}
