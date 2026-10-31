// Package ch04stateless ports the code of Chapters 3 and 4 of the PropEr
// Testing book: the running example biggest, the sort and last properties,
// and the solutions of the exercises.
package ch04stateless

import (
	"cmp"
	"slices"
	"strings"
)

// Biggest returns the largest element of a non-empty slice.
func Biggest(xs []int) int {
	m := xs[0]
	for _, x := range xs[1:] {
		if x >= m { // >=, so that equal elements do not matter either way
			m = x
		}
	}
	return m
}

// BiggestHead is the first wrong attempt of the chapter: it returns the head.
func BiggestHead(xs []int) int { return xs[0] }

// Seq returns start, start+1, ..., start+count, like lists:seq(Start, Start+Count).
func Seq(start, count int) []int {
	out := make([]int, 0, count+1)
	for i := 0; i <= count; i++ {
		out = append(out, start+i)
	}
	return out
}

// Pair is a key-value pair for the keysort properties.
type Pair struct {
	Key int
	Val string
}

// KeySort sorts pairs by key, keeping the order of pairs with equal keys, as
// lists:keysort/2 does.
func KeySort(ps []Pair) []Pair {
	out := slices.Clone(ps)
	slices.SortStableFunc(out, func(a, b Pair) int { return cmp.Compare(a.Key, b.Key) })
	return out
}

// Set is a set of ints, the system under test of exercise 5.
type Set map[int]struct{}

// SetFrom builds a set from a slice.
func SetFrom(xs []int) Set {
	s := Set{}
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

// Union returns the union of two sets.
func Union(a, b Set) Set {
	out := Set{}
	for x := range a {
		out[x] = struct{}{}
	}
	for x := range b {
		out[x] = struct{}{}
	}
	return out
}

// Sorted returns the elements of the set in ascending order.
func (s Set) Sorted() []int {
	out := make([]int, 0, len(s))
	for x := range s {
		out = append(out, x)
	}
	slices.Sort(out)
	return out
}

// Merge merges two maps; on a conflict the value of the first map wins, as
// the merge function of exercise 6 does.
func Merge(a, b map[string]int) map[string]int {
	out := make(map[string]int, len(a)+len(b))
	for k, v := range b {
		out[k] = v
	}
	for k, v := range a {
		out[k] = v
	}
	return out
}

// WordCount counts the words of a string, treating runs of spaces as
// separators, as the solution of exercise 7 does.
func WordCount(s string) int {
	s = strings.Trim(s, " ")
	if s == "" {
		return 0
	}
	spaces := 0
	prev := false
	for _, r := range s {
		if r == ' ' {
			if !prev {
				spaces++
			}
			prev = true
		} else {
			prev = false
		}
	}
	return spaces + 1
}

// AltWordCount is the alternative implementation the exercise compares
// against: a scanner that alternates between words and spaces.
func AltWordCount(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		switch {
		case r != ' ' && !inWord:
			count++
			inWord = true
		case r == ' ':
			inWord = false
		}
	}
	return count
}
