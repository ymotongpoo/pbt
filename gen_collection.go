// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import "fmt"

// maxDistinctTries is how many values a distinct collection draws for one
// position before it stops growing.
const maxDistinctTries = 10

// SliceOf generates slices of any length. The expected length grows with the
// size parameter.
func SliceOf[E any](elem Gen[E]) Gen[[]E] {
	return named(SliceOfN(elem, 0, -1), "SliceOf("+elem.name+")")
}

// SliceOfN generates slices with between minLen and maxLen elements. A
// negative maxLen means no upper bound.
func SliceOfN[E any](elem Gen[E], minLen, maxLen int) Gen[[]E] {
	checkLengths(minLen, maxLen)
	return Gen[[]E]{name: fmt.Sprintf("SliceOfN(%s, %d, %d)", elem.name, minLen, maxLen), draw: func(t *T) []E {
		return drawSlice(t, elem, minLen, maxLen, nil)
	}}
}

// SliceOfDistinct generates slices whose elements have distinct keys.
func SliceOfDistinct[E any, K comparable](elem Gen[E], key func(E) K) Gen[[]E] {
	return named(SliceOfNDistinct(elem, 0, -1, key), "SliceOfDistinct("+elem.name+")")
}

// SliceOfNDistinct generates slices with between minLen and maxLen elements
// whose keys are distinct. The test case is discarded when minLen distinct
// elements cannot be found.
func SliceOfNDistinct[E any, K comparable](elem Gen[E], minLen, maxLen int, key func(E) K) Gen[[]E] {
	checkLengths(minLen, maxLen)
	return Gen[[]E]{name: fmt.Sprintf("SliceOfNDistinct(%s, %d, %d)", elem.name, minLen, maxLen), draw: func(t *T) []E {
		seen := map[K]struct{}{}
		return drawSlice(t, elem, minLen, maxLen, func(e E) bool {
			k := key(e)
			if _, dup := seen[k]; dup {
				return false
			}
			seen[k] = struct{}{}
			return true
		})
	}}
}

// MapOf generates maps of any length.
func MapOf[K comparable, V any](key Gen[K], val Gen[V]) Gen[map[K]V] {
	return named(MapOfN(key, val, 0, -1), fmt.Sprintf("MapOf(%s, %s)", key.name, val.name))
}

// MapOfN generates maps with between minLen and maxLen entries. The test
// case is discarded when minLen distinct keys cannot be found.
func MapOfN[K comparable, V any](key Gen[K], val Gen[V], minLen, maxLen int) Gen[map[K]V] {
	checkLengths(minLen, maxLen)
	type entry struct {
		k K
		v V
	}
	pair := Gen[entry]{name: "entry", draw: func(t *T) entry {
		return entry{k: key.drawSpan(t), v: val.drawSpan(t)}
	}}
	return Gen[map[K]V]{name: fmt.Sprintf("MapOfN(%s, %s, %d, %d)", key.name, val.name, minLen, maxLen), draw: func(t *T) map[K]V {
		m := make(map[K]V)
		drawSlice(t, pair, minLen, maxLen, func(e entry) bool {
			if _, dup := m[e.k]; dup {
				return false
			}
			m[e.k] = e.v
			return true
		})
		return m
	}}
}

// SampledFrom picks one of the values with equal probability. It shrinks
// toward the first one, as PropEr's elements/1 does.
func SampledFrom[E any](values []E) Gen[E] {
	if len(values) == 0 {
		panic("pbt: SampledFrom needs at least one value")
	}
	return Gen[E]{name: fmt.Sprintf("SampledFrom(%d values)", len(values)), draw: func(t *T) E {
		return values[t.draw(uint64(len(values)-1), plainMeta, modeUniform)]
	}}
}

func checkLengths(minLen, maxLen int) {
	if minLen < 0 || (maxLen >= 0 && maxLen < minLen) {
		panic(fmt.Sprintf("pbt: invalid length bounds %d, %d", minLen, maxLen))
	}
}

// drawSlice generates a slice element by element. Each element is preceded
// by the choice to continue, and both sit in one span, so that deleting the
// span removes exactly one element. Elements rejected by accept are retried
// in their own spans, which the shrinker can delete as well.
func drawSlice[E any](t *T, elem Gen[E], minLen, maxLen int, accept func(E) bool) []E {
	out := make([]E, 0)
	p := continueProbability(t, maxLen)
	for {
		t.beginSpan("item")
		var more bool
		switch {
		case len(out) < minLen:
			t.forced(1)
			more = true
		case maxLen >= 0 && len(out) >= maxLen:
			t.forced(0)
		default:
			more = t.drawBool(p)
		}
		if !more {
			t.endSpan()
			return out
		}
		e, ok := drawAccepted(t, elem, accept)
		t.endSpan()
		if !ok {
			if len(out) >= minLen {
				return out
			}
			t.discard(fmt.Sprintf("%s: could not generate %d distinct elements", elem.name, minLen))
		}
		out = append(out, e)
	}
}

func drawAccepted[E any](t *T, elem Gen[E], accept func(E) bool) (E, bool) {
	if accept == nil {
		return elem.drawSpan(t), true
	}
	for range maxDistinctTries {
		if e := elem.drawSpan(t); accept(e) {
			return e, true
		}
	}
	var zero E
	return zero, false
}

// continueProbability makes the expected length of a collection track the
// size parameter, as PropEr's list generators do.
func continueProbability(t *T, maxLen int) float64 {
	avg := float64(t.size)
	if maxLen >= 0 && float64(maxLen) < avg {
		avg = float64(maxLen)
	}
	if avg < 1 {
		avg = 1
	}
	return avg / (avg + 1)
}

// asciiShrinkOrder lists printable ASCII from the simplest rune to the most
// complex one; Rune shrinks along it.
const asciiShrinkOrder = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

var (
	asciiRunes   = []rune(asciiShrinkOrder)
	controlRunes = append([]rune{'\n', '\t', '\r', 0}, controls()...)
)

// runeTableMax is the largest rune choice: the two tables followed by every
// code point from 0x80 up, with the surrogate range skipped.
const runeTableMax = uint64(len(asciiShrinkOrder)+33) + (0x10FFFF - 0x80 - (0xE000 - 0xD800)) - 1

func controls() []rune {
	var rs []rune
	for r := rune(1); r < 0x20; r++ {
		if r != '\n' && r != '\t' && r != '\r' {
			rs = append(rs, r)
		}
	}
	return append(rs, 0x7f)
}

// Rune generates valid Unicode code points, mostly printable ASCII, with
// some control characters and some from the rest of the code space, in one
// choice: the printable table first, then the control table, then the
// remaining code points. It shrinks toward '0'.
func Rune() Gen[rune] {
	return Gen[rune]{name: "Rune()", draw: func(t *T) rune {
		u := t.draw(runeTableMax, plainMeta, modeText)
		switch tables := uint64(len(asciiRunes) + len(controlRunes)); {
		case u < uint64(len(asciiRunes)):
			return asciiRunes[u]
		case u < tables:
			return controlRunes[u-uint64(len(asciiRunes))]
		default:
			const surrogates = 0xE000 - 0xD800
			r := rune(0x80 + (u - tables))
			if r >= 0xD800 {
				r += surrogates
			}
			return r
		}
	}}
}

// String generates valid UTF-8 strings of runes from Rune.
func String() Gen[string] { return named(StringOf(Rune()), "String()") }

// StringN generates strings with between minLen and maxLen runes.
func StringN(minLen, maxLen int) Gen[string] {
	return named(StringOfN(Rune(), minLen, maxLen), fmt.Sprintf("StringN(%d, %d)", minLen, maxLen))
}

// StringOf generates strings whose runes come from r.
func StringOf(r Gen[rune]) Gen[string] {
	return named(StringOfN(r, 0, -1), "StringOf("+r.name+")")
}

// StringOfN generates strings with between minLen and maxLen runes from r.
func StringOfN(r Gen[rune], minLen, maxLen int) Gen[string] {
	return named(Map(SliceOfN(r, minLen, maxLen), func(rs []rune) string { return string(rs) }),
		fmt.Sprintf("StringOfN(%s, %d, %d)", r.name, minLen, maxLen))
}
