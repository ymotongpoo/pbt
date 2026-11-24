// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode"
	"unicode/utf8"
)

// reNode is a parsed regular expression prepared for generation: the
// operator, and for literals and character classes the runes to pick from.
type reNode struct {
	op       syntax.Op
	variants [][]rune    // OpLiteral: the case variants of each rune, lower case first
	ranges   []runeRange // character classes: the ranges, without surrogates
	ascii    []rune      // character classes: the printable ASCII members, in Rune's shrink order
	subs     []*reNode
	min, max int  // OpRepeat and the repetition operators; max -1 is unbounded
	live     bool // some string matches the node, ignoring assertions
}

// reASCIIWeights splits the draws of a character class between its
// printable ASCII members and all of its members.
var reASCIIWeights = []int{3, 1}

// StringMatching generates strings that the regular expression matches as a
// whole: for every value s, regexp.MustCompile(`\A(?:` + pattern + `)\z`)
// matches s. The pattern uses the syntax of package regexp, and StringMatching
// panics when it does not parse or can match nothing.
//
// The string is built from the syntax tree, so shrinking keeps it matching:
// an alternation shrinks toward its first branch, a repetition toward its
// minimum count, a literal under (?i) toward lower case, and
// a character class toward its first printable ASCII member in the order
// Rune uses, or toward its first code point when it has none. Three
// quarters of the characters drawn for a class with printable ASCII
// members are among those members, so that values stay readable; the rest
// come from the whole class, surrogates excluded. Unbounded repetitions
// such as * and + grow with the size parameter, like SliceOf.
//
// The empty-width assertions ^, $, \A, \z, \b and \B generate nothing.
// Placed at the ends of the pattern they hold anyway; a value that breaks
// one in the middle, as \b between two letters does, is drawn again, up to
// the same number of tries as Filter, and the case is discarded after
// that.
func StringMatching(pattern string) Gen[string] {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		panic(fmt.Sprintf("pbt: StringMatching(%q): %v", pattern, err))
	}
	root := compileRegexp(re)
	if !root.live {
		panic(fmt.Sprintf("pbt: StringMatching(%q): the pattern matches no string", pattern))
	}
	whole := regexp.MustCompile(`\A(?:` + pattern + `)\z`)
	name := fmt.Sprintf("StringMatching(%q)", pattern)
	return Gen[string]{name: name, draw: func(t *T) string {
		var b strings.Builder
		for range maxFilterTries {
			b.Reset()
			t.beginSpan("try")
			genRegexp(t, root, &b)
			t.endSpan()
			if s := b.String(); whole.MatchString(s) {
				return s
			}
		}
		t.discard(name + ": no generated string satisfied the assertions in " + fmt.Sprint(maxFilterTries) + " tries")
		panic("unreachable")
	}}
}

func compileRegexp(re *syntax.Regexp) *reNode {
	n := &reNode{op: re.Op, min: re.Min, max: re.Max}
	switch re.Op {
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			vs := []rune{r}
			if re.Flags&syntax.FoldCase != 0 {
				// The parser keeps one case of a folded literal, not
				// the one written, so the lower case goes first.
				lower := unicode.ToLower(r)
				vs[0] = lower
				for f := unicode.SimpleFold(lower); f != lower; f = unicode.SimpleFold(f) {
					vs = append(vs, f)
				}
			}
			n.variants = append(n.variants, vs)
		}
	case syntax.OpCharClass:
		n.setClass(re.Rune)
	case syntax.OpAnyChar:
		n.op = syntax.OpCharClass
		n.setClass([]rune{0, utf8.MaxRune})
	case syntax.OpAnyCharNotNL:
		n.op = syntax.OpCharClass
		n.setClass([]rune{0, '\n' - 1, '\n' + 1, utf8.MaxRune})
	case syntax.OpStar:
		n.op, n.min, n.max = syntax.OpRepeat, 0, -1
	case syntax.OpPlus:
		n.op, n.min, n.max = syntax.OpRepeat, 1, -1
	case syntax.OpQuest:
		n.op, n.min, n.max = syntax.OpRepeat, 0, 1
	}
	for _, s := range re.Sub {
		n.subs = append(n.subs, compileRegexp(s))
	}
	if n.op == syntax.OpAlternate {
		// Only the branches that can match are candidates, so a branch
		// that matches nothing is never drawn.
		live := n.subs[:0]
		for _, s := range n.subs {
			if s.live {
				live = append(live, s)
			}
		}
		n.subs = live
	}
	n.live = reCanMatch(n)
	return n
}

// setClass takes the pairs of a character class, lo and hi alternately.
func (n *reNode) setClass(pairs []rune) {
	for i := 0; i+1 < len(pairs); i += 2 {
		n.ranges = appendRange(n.ranges, pairs[i], pairs[i+1], 1)
	}
	for _, r := range asciiShrinkOrder {
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i] <= r && r <= pairs[i+1] {
				n.ascii = append(n.ascii, r)
				break
			}
		}
	}
}

// reCanMatch reports whether some string matches n, ignoring assertions,
// from the live flags of its children.
func reCanMatch(n *reNode) bool {
	switch n.op {
	case syntax.OpNoMatch:
		return false
	case syntax.OpCharClass:
		return len(n.ranges) > 0
	case syntax.OpRepeat:
		return n.min == 0 || n.subs[0].live
	case syntax.OpAlternate:
		return len(n.subs) > 0
	}
	for _, s := range n.subs {
		if !s.live {
			return false
		}
	}
	return true
}

func genRegexp(t *T, n *reNode, b *strings.Builder) {
	switch n.op {
	case syntax.OpLiteral:
		for _, vs := range n.variants {
			r := vs[0]
			if len(vs) > 1 {
				r = vs[t.draw(uint64(len(vs)-1), plainMeta, modeUniform)]
			}
			b.WriteRune(r)
		}
	case syntax.OpCharClass:
		b.WriteRune(drawClass(t, n))
	case syntax.OpCapture:
		genRegexp(t, n.subs[0], b)
	case syntax.OpConcat:
		for _, s := range n.subs {
			genRegexp(t, s, b)
		}
	case syntax.OpAlternate:
		s := n.subs[t.draw(uint64(len(n.subs)-1), flagMeta, modeUniform)]
		t.beginSpan("branch")
		genRegexp(t, s, b)
		t.endSpan()
	case syntax.OpRepeat:
		genRepeat(t, n, b)
	}
	// OpEmptyMatch and the assertions write nothing.
}

// genRepeat writes between n.min and n.max repetitions, each in its own
// span behind the choice to continue, the way drawSlice builds a slice, so
// that the shrinker deletes whole repetitions.
func genRepeat(t *T, n *reNode, b *strings.Builder) {
	sub := n.subs[0]
	maxN := n.max
	if !sub.live {
		maxN = 0
	}
	p := continueProbability(t, maxN)
	for i := 0; ; i++ {
		t.beginSpan("item")
		var more bool
		switch {
		case i < n.min:
			t.forced(1)
			more = true
		case maxN >= 0 && i >= maxN:
			t.forced(0)
		default:
			more = t.drawBool(p)
		}
		if !more {
			t.endSpan()
			return
		}
		genRegexp(t, sub, b)
		t.endSpan()
	}
}

func drawClass(t *T, n *reNode) rune {
	if len(n.ascii) > 0 && t.drawIndex(reASCIIWeights, 4) == 0 {
		return n.ascii[t.draw(uint64(len(n.ascii)-1), plainMeta, modeUniform)]
	}
	r := n.ranges[t.draw(uint64(len(n.ranges)-1), plainMeta, modeUniform)]
	return r.lo + rune(t.draw(r.n-1, plainMeta, modeBiased))
}
