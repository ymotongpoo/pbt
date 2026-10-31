package pbt

import (
	"iter"
	"time"
)

// NoShrink generates the values of g but keeps the shrinker from changing
// them, as PropEr's noshrink/1 does. A collection that contains the value can
// still drop it.
func NoShrink[V any](g Gen[V]) Gen[V] {
	return Gen[V]{name: "NoShrink(" + g.name + ")", draw: func(t *T) V {
		t.spans[t.open[len(t.open)-1]].noshrink = true
		return g.draw(t)
	}}
}

// LetShrink draws every part and combines them, and lets the result shrink
// to any single part, as PropEr's ?LETSHRINK does. It is what makes a tree
// shrink to one of its subtrees or a concatenation to one of its pieces.
func LetShrink[V any](parts []Gen[V], combine func([]V) V) Gen[V] {
	if len(parts) == 0 {
		panic("pbt: LetShrink needs at least one part")
	}
	weights := make([]int, len(parts)+1)
	weights[len(parts)] = 1
	return Gen[V]{name: "LetShrink(" + joinNames(parts) + ")", draw: func(t *T) V {
		which := t.drawIndex(weights, 1)
		vals := make([]V, len(parts))
		for i, p := range parts {
			vals[i] = p.drawSpan(t)
		}
		if which < len(parts) {
			return vals[which]
		}
		return combine(vals)
	}}
}

// SampleShrink generates a value of g and then yields the values that
// successive shrinking steps produce when every value counts as failing, as
// proper_gen:sampleshrink does. It shows what a generator shrinks toward.
func SampleShrink[V any](g Gen[V], seed ...uint64) iter.Seq[V] {
	return func(yield func(V) bool) {
		s := randomSeed()
		if len(seed) > 0 {
			s = seed[0]
		}
		var cur V
		r := &runner{
			tb:   nopTB{},
			prop: func(t *T) { cur = g.Draw(t, ""); t.Fail() },
			cfg:  config{maxSize: defaultMaxSize, shrinkTime: defaultShrink},
		}
		t := r.run(nil, exampleSize, newRand(s))
		if t.discarded || !yield(cur) {
			return
		}
		sh := &shrinker{r: r, cur: t, deadline: time.Now().Add(defaultShrink), onAccept: func() bool { return yield(cur) }}
		sh.run()
	}
}

type nopTB struct{}

func (nopTB) Helper()               {}
func (nopTB) Name() string          { return "" }
func (nopTB) Logf(string, ...any)   {}
func (nopTB) Errorf(string, ...any) {}
