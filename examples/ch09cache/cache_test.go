// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch09cache

import (
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

const cacheSize = 10

// harness holds the cache under test and the model side by side. The model
// mirrors the cache: the same bounded list of entries. The commands are
// closures over the harness, so a test can run one program against several
// caches by swapping the pointer.
type harness struct {
	cache *Cache
	model []entry
}

func (h *harness) reset() { h.model = nil }

func (h *harness) find(key int) (int, bool) {
	for _, e := range h.model {
		if e.key == key {
			return e.val, true
		}
	}
	return 0, false
}

func (h *harness) put(key, val int) {
	for i := range h.model {
		if h.model[i].key == key {
			h.model[i].val = val
			return
		}
	}
	if len(h.model) >= cacheSize {
		h.model = h.model[1:]
	}
	h.model = append(h.model, entry{key, val})
}

type found struct {
	val int
	ok  bool
}

// Section 9.4: keys mix a small range, so that they collide, with any
// integer, so that the cache fills up; values are any integer. The
// frequencies favour writes, as the chapter does.
var (
	key = pbt.OneOf(pbt.IntRange(1, cacheSize), pbt.Int())
	val = pbt.Int()
)

func (h *harness) commands() pbt.Gen[pbt.Command] {
	return pbt.Frequency(
		pbt.W(1, pbt.Map(key, func(k int) pbt.Command {
			return pbt.Command{Name: "find", Args: []any{k},
				Run: func(pbt.Env) any { v, ok := h.cache.Find(k); return found{v, ok} },
				Post: func(_ pbt.Env, res any) error {
					v, ok := h.find(k)
					if res != (found{v, ok}) {
						return fmt.Errorf("find(%d) = %v, model has %v", k, res, found{v, ok})
					}
					return nil
				}}
		})),
		pbt.W(3, pbt.Custom("put", func(t *pbt.T) pbt.Command {
			k, v := key.Draw(t, ""), val.Draw(t, "")
			return pbt.Command{Name: "put", Args: []any{k, v},
				Run:  func(pbt.Env) any { h.cache.Put(k, v); return nil },
				Next: func(pbt.Var) { h.put(k, v) }}
		})),
		pbt.W(1, pbt.Just(pbt.Command{Name: "flush",
			Run:  func(pbt.Env) any { h.cache.Flush(); return nil },
			Next: func(pbt.Var) { h.model = nil }})),
	)
}

func (h *harness) program() pbt.Gen[pbt.Program] {
	return pbt.Commands(h.reset, h.commands, pbt.ModelState(func() any { return h.model }))
}

func TestSequential(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{cache: New(cacheSize)}
		h.program().Draw(t, "cmds").RunSequential(t)
	})
}

// The chapter injects a bug by making the cache one entry too small. The
// counterexample needs cacheSize puts of distinct keys and one find.
func TestUndersizedCacheIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		h := &harness{cache: New(cacheSize - 1)}
		h.program().Draw(t, "cmds").RunSequential(t)
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
		h := &harness{cache: New(cacheSize)}
		pbt.ParallelCommands(h.reset, h.commands).Draw(t, "cmds").RunParallel(t)
	})
}

func TestParallelCatchesTheRace(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		h := &harness{}
		prog := pbt.ParallelCommands(h.reset, h.commands).Draw(t, "cmds")
		// Repeat the parallel run a few times per case, as the chapter
		// reruns its property many times with an aggressive scheduler.
		for range 5 {
			h.cache = NewRacy(cacheSize)
			prog.RunParallel(t)
		}
	}, pbt.Seed(1), pbt.Checks(1000))
	if !strings.Contains(report, "no interleaving") {
		t.Errorf("expected a linearizability failure:\n%s", report)
	}
}
