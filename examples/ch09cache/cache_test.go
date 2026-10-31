package ch09cache

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

const cacheSize = 10

// model mirrors the cache: the same bounded list of entries, kept
// immutable so that generation and execution evolve it identically.
type model struct {
	entries []entry
}

func (m model) find(key int) (int, bool) {
	for _, e := range m.entries {
		if e.key == key {
			return e.val, true
		}
	}
	return 0, false
}

func (m model) put(key, val int) model {
	entries := slices.Clone(m.entries)
	for i := range entries {
		if entries[i].key == key {
			entries[i].val = val
			return model{entries}
		}
	}
	if len(entries) >= cacheSize {
		entries = entries[1:]
	}
	return model{append(entries, entry{key, val})}
}

type found struct {
	val int
	ok  bool
}

type cmd = pbt.Cmd[model, *Cache]

// Section 9.4: keys mix a small range, so that they collide, with any
// integer, so that the cache fills up; values are any integer. The
// frequencies favour writes, as the chapter does.
var (
	key = pbt.OneOf(pbt.IntRange(1, cacheSize), pbt.Int())
	val = pbt.Int()
)

func commands(model) pbt.Gen[pbt.Command[model, *Cache]] {
	return pbt.Frequency(
		pbt.Weighted[pbt.Command[model, *Cache]]{Weight: 1, Gen: pbt.Map(key, func(k int) pbt.Command[model, *Cache] {
			return cmd{Name: "find", Args: []any{k},
				Run: func(c *Cache, _ pbt.Env) any { v, ok := c.Find(k); return found{v, ok} },
				Post: func(m model, _ pbt.Env, res any) error {
					v, ok := m.find(k)
					if res != (found{v, ok}) {
						return fmt.Errorf("find(%d) = %v, model has %v", k, res, found{v, ok})
					}
					return nil
				}}
		})},
		pbt.Weighted[pbt.Command[model, *Cache]]{Weight: 3, Gen: pbt.Custom("put", func(t *pbt.T) pbt.Command[model, *Cache] {
			k, v := key.Draw(t, ""), val.Draw(t, "")
			return cmd{Name: "put", Args: []any{k, v},
				Run:  func(c *Cache, _ pbt.Env) any { c.Put(k, v); return nil },
				Next: func(m model, _ pbt.Var) model { return m.put(k, v) }}
		})},
		pbt.Weighted[pbt.Command[model, *Cache]]{Weight: 1, Gen: pbt.Just[pbt.Command[model, *Cache]](cmd{Name: "flush",
			Run:  func(c *Cache, _ pbt.Env) any { c.Flush(); return nil },
			Next: func(model, pbt.Var) model { return model{} }})},
	)
}

func TestSequential(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, New(cacheSize))
	})
}

// The chapter injects a bug by making the cache one entry too small. The
// counterexample needs cacheSize puts of distinct keys and one find.
func TestUndersizedCacheIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, New(cacheSize-1))
	}, pbt.Seed(1), pbt.Checks(500))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "= put(") != cacheSize || strings.Count(program, "= find(") != 1 {
		t.Errorf("expected %d puts and one find:\n%s", cacheSize, report)
	}
}

// Section 9.5: the parallel test. The racy cache lets two writers both see
// room for one more entry, so the cache grows past its capacity and a
// later find disagrees with every interleaving of the model.
func TestParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(model{}, commands).Draw(t, "cmds")
		prog.RunParallel(t, New(cacheSize))
	})
}

func TestParallelCatchesTheRace(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(model{}, commands).Draw(t, "cmds")
		// Repeat the parallel run a few times per case, as the chapter
		// reruns its property many times with an aggressive scheduler.
		for range 5 {
			prog.RunParallel(t, NewRacy(cacheSize))
		}
	}, pbt.Seed(1), pbt.Checks(1000))
	if !strings.Contains(report, "no interleaving") {
		t.Errorf("expected a linearizability failure:\n%s", report)
	}
}
