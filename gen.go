// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"strings"
	"sync"
)

// maxFilterTries is how many values Filter draws before it discards the test
// case. PropEr calls the same limit constraint_tries.
const maxFilterTries = 100

// exampleSize is the size parameter Example and SampleShrink use, the same
// default as proper_gen:pick.
const exampleSize = 10

// Gen generates values of type V. A generator is a deterministic function of
// the choices it draws from a *T, which is what makes every generator
// shrinkable without extra code.
type Gen[V any] struct {
	name   string
	nameFn func() string // set by Lazy, whose name is known only once resolved
	draw   func(*T) V
}

// String returns a description of the generator, used in reports.
func (g Gen[V]) String() string {
	if g.nameFn != nil {
		return g.nameFn()
	}
	return g.name
}

// Draw generates a value. The label names the value in the report of a
// failing test case; pass "" to use the generator's description. A draw at
// the top level of the property always appears in the report; a draw inside
// a generator appears when it has a label, indented under the value it
// contributes to, which is how the report shows how a value was built.
func (g Gen[V]) Draw(t *T, label string) V {
	v := g.drawSpan(t)
	if depth := len(t.open); depth == 0 || label != "" {
		t.draws = append(t.draws, drawRecord{label: label, gen: g.String(), value: v, depth: depth})
	}
	return v
}

// drawSpan generates a value inside a span labeled with the generator's
// description, so that the shrinker can treat the choices as one unit.
func (g Gen[V]) drawSpan(t *T) V {
	t.beginSpan(g.String())
	defer t.endSpan()
	return g.draw(t)
}

// Example returns a sample value, as proper_gen:pick does. Without a seed the
// sample differs between calls.
func (g Gen[V]) Example(seed ...uint64) V {
	s := randomSeed()
	if len(seed) > 0 {
		s = seed[0]
	}
	for attempt := uint64(0); ; attempt++ {
		t := newT("", nil, exampleSize, defaultMaxSize, newRand(s+attempt))
		if v, ok := tryDraw(t, g); ok {
			return v
		}
		if attempt == maxFilterTries {
			panic(fmt.Sprintf("pbt: %s.Example: every attempt was discarded", g))
		}
	}
}

func tryDraw[V any](t *T, g Gen[V]) (v V, ok bool) {
	defer func() {
		if rec := recover(); rec != nil {
			if _, discarded := rec.(discardTest); !discarded {
				panic(rec)
			}
		}
	}()
	return g.drawSpan(t), true
}

func named[V any](g Gen[V], name string) Gen[V] {
	g.name = name
	g.nameFn = nil
	return g
}

// Custom wraps a function that draws from other generators into a generator.
// The name appears in reports.
func Custom[V any](name string, f func(*T) V) Gen[V] {
	return Gen[V]{name: name, draw: f}
}

// Just always generates v. It shrinks to nothing.
func Just[V any](v V) Gen[V] {
	return Gen[V]{name: fmt.Sprintf("Just(%#v)", v), draw: func(*T) V { return v }}
}

// Map applies f to every value of g.
func Map[A, B any](g Gen[A], f func(A) B) Gen[B] {
	return Gen[B]{name: "Map(" + g.name + ")", draw: func(t *T) B { return f(g.drawSpan(t)) }}
}

// Bind draws a value from g and then from the generator f returns for it.
// It is the equivalent of PropEr's ?LET. Shrinking works across the bind:
// the first value shrinks and the second is regenerated from the same choices.
func Bind[A, B any](g Gen[A], f func(A) Gen[B]) Gen[B] {
	return Gen[B]{name: "Bind(" + g.name + ")", draw: func(t *T) B { return f(g.drawSpan(t)).drawSpan(t) }}
}

// Filter keeps the values of g for which pred holds. It is the equivalent of
// PropEr's ?SUCHTHAT. After maxFilterTries rejected values it discards the
// test case, so predicates should accept a good share of the values.
func Filter[V any](g Gen[V], pred func(V) bool) Gen[V] {
	name := "Filter(" + g.name + ")"
	return Gen[V]{name: name, draw: func(t *T) V {
		for range maxFilterTries {
			if v := g.drawSpan(t); pred(v) {
				return v
			}
		}
		t.discard(name + ": no value satisfied the predicate in " + fmt.Sprint(maxFilterTries) + " tries")
		panic("unreachable")
	}}
}

// OneOf picks one of the generators with equal probability. It shrinks
// toward the first one.
func OneOf[V any](gens ...Gen[V]) Gen[V] {
	if len(gens) == 0 {
		panic("pbt: OneOf needs at least one generator")
	}
	return Gen[V]{name: "OneOf(" + joinNames(gens) + ")", draw: func(t *T) V {
		return gens[t.draw(uint64(len(gens)-1), flagMeta, modeUniform)].drawSpan(t)
	}}
}

// Weighted pairs a generator with its weight for Frequency.
type Weighted[V any] struct {
	Weight int
	Gen    Gen[V]
}

// Frequency picks one of the generators with probability proportional to its
// weight, as PropEr's frequency/1 does. Weights decide generation only:
// shrinking moves toward the first alternative regardless of weight, and an
// alternative with weight 0 is never generated but can be reached by
// shrinking.
func Frequency[V any](alts ...Weighted[V]) Gen[V] {
	if len(alts) == 0 {
		panic("pbt: Frequency needs at least one alternative")
	}
	weights := make([]int, len(alts))
	gens := make([]Gen[V], len(alts))
	total := 0
	for i, a := range alts {
		if a.Weight < 0 {
			panic(fmt.Sprintf("pbt: Frequency: negative weight %d", a.Weight))
		}
		weights[i] = a.Weight
		gens[i] = a.Gen
		total += a.Weight
	}
	if total == 0 {
		panic("pbt: Frequency: all weights are 0")
	}
	return Gen[V]{name: "Frequency(" + joinNames(gens) + ")", draw: func(t *T) V {
		return gens[t.drawIndex(weights, total)].drawSpan(t)
	}}
}

// ShrinkTo generates values of g but lets them shrink to values of the
// alternatives, in order. It is the equivalent of PropEr's ?SHRINK and
// re-centers shrinking, for example from a range of years toward 1970.
func ShrinkTo[V any](g Gen[V], alts ...Gen[V]) Gen[V] {
	ws := make([]Weighted[V], 0, len(alts)+1)
	for _, a := range alts {
		ws = append(ws, Weighted[V]{Weight: 0, Gen: a})
	}
	ws = append(ws, Weighted[V]{Weight: 1, Gen: g})
	return named(Frequency(ws...), "ShrinkTo("+g.name+")")
}

// Sized builds the generator from the size parameter of the test case, as
// PropEr's ?SIZED does.
func Sized[V any](f func(size int) Gen[V]) Gen[V] {
	return Gen[V]{name: "Sized(...)", draw: func(t *T) V { return f(t.size).drawSpan(t) }}
}

// Resize runs g with a fixed size parameter, as PropEr's resize/2 does.
func Resize[V any](size int, g Gen[V]) Gen[V] {
	return Gen[V]{name: fmt.Sprintf("Resize(%d, %s)", size, g.name), draw: func(t *T) V {
		saved := t.size
		t.size = size
		defer func() { t.size = saved }()
		return g.drawSpan(t)
	}}
}

// Lazy defers the construction of g until it is first used, which is how a
// generator refers to itself in a recursive definition. It is the equivalent
// of PropEr's ?LAZY.
func Lazy[V any](f func() Gen[V]) Gen[V] {
	resolve := sync.OnceValue(f)
	return Gen[V]{
		name:   "Lazy",
		nameFn: func() string { return resolve().String() },
		draw:   func(t *T) V { return resolve().draw(t) },
	}
}

func joinNames[V any](gens []Gen[V]) string {
	names := make([]string, len(gens))
	for i, g := range gens {
		names[i] = g.name
	}
	return strings.Join(names, ", ")
}
