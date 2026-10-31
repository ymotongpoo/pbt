// Package ch05generators ports Chapter 5 of the PropEr Testing book: the
// generators that make properties more precise, the statistics that show
// what they produce, and the exercises on recursion, timestamps and
// symbolic construction.
package ch05generators

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/ch04stateless"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

// Section 5.1: the first word counter only knew about single spaces. A
// generator that builds strings from words and splitters, and knows how
// many words it put in, gives the property an expected count.

var splitter = pbt.StringOfN(pbt.Just(' '), 1, -1)

var word = pbt.StringOfN(pbt.SampledFrom([]rune("abcdefghijklmnopqrstuvwxyz0123456789")), 1, -1)

type wordy struct {
	text  string
	words int
}

// wordyString is the recursive generator of Section 5.8, with Lazy where the
// chapter needs ?LAZY. Each level adds a word and a splitter; the leaf is a
// splitter alone.
func wordyString() pbt.Gen[wordy] {
	return pbt.Frequency(
		pbt.Weighted[wordy]{Weight: 1, Gen: pbt.Map(splitter, func(s string) wordy { return wordy{s, 0} })},
		pbt.Weighted[wordy]{Weight: 3, Gen: pbt.Custom("word+rest", func(t *pbt.T) wordy {
			w := word.Draw(t, "")
			sp := splitter.Draw(t, "")
			rest := pbt.Lazy(wordyString).Draw(t, "")
			return wordy{w + sp + rest.text, rest.words + 1}
		})},
	)
}

// wordyStringDeterministic is the Sized version of the same generator: the
// size parameter bounds the number of words, so nothing recurses.
var wordyStringDeterministic = pbt.Sized(func(size int) pbt.Gen[wordy] {
	return pbt.Custom("wordy", func(t *pbt.T) wordy {
		n := pbt.IntRange(0, size).Draw(t, "")
		var b strings.Builder
		b.WriteString(splitter.Draw(t, ""))
		for range n {
			b.WriteString(word.Draw(t, ""))
			b.WriteString(splitter.Draw(t, ""))
		}
		return wordy{b.String(), n}
	})
})

func TestWordCountWithWordyStrings(t *testing.T) {
	for _, gen := range []pbt.Gen[wordy]{wordyString(), wordyStringDeterministic} {
		pbt.Check(t, pbt.ForAll(gen, func(t *pbt.T, w wordy) {
			t.Collect(w.words)
			if got := ch04stateless.WordCount(w.text); got != w.words {
				t.Fatalf("WordCount(%q) = %d, want %d", w.text, got, w.words)
			}
		}))
	}
}

// Section 5.2: collect and aggregate show the distribution. One property
// collects raw sizes, the next groups them in buckets of ten, and the third
// aggregates the cards of a five-card hand.
func TestCollectSizes(t *testing.T) {
	pbt.Check(t, pbt.ForAll(pbt.Bytes(), func(t *pbt.T, b []byte) {
		t.Collect(len(b))
	}))
	pbt.Check(t, pbt.ForAll(pbt.Bytes(), func(t *pbt.T, b []byte) {
		t.Collect(toRange(10, len(b)))
	}))
}

// toRange buckets n into the label "lo-hi" of width m, like to_range/2.
func toRange(m, n int) string {
	base := n / m
	return fmt.Sprintf("%d-%d", base*m, (base+1)*m)
}

type card struct {
	suit string
	rank int
}

var cardGen = pbt.Custom("card", func(t *pbt.T) card {
	return card{pbt.SampledFrom([]string{"club", "diamond", "heart", "spade"}).Draw(t, ""), pbt.IntRange(1, 13).Draw(t, "")}
})

func TestAggregateHands(t *testing.T) {
	pbt.Check(t, pbt.ForAll(pbt.SliceOfN(cardGen, 5, 5), func(t *pbt.T, hand []card) {
		for _, c := range hand {
			t.Aggregate(c)
		}
	}))
}

// Section 5.3: Resize fixes the size parameter and Sized reads it. The
// profile's biography grows with the size, 35 characters per unit.
func TestResize(t *testing.T) {
	pbt.Check(t, pbt.ForAll(pbt.Resize(150, pbt.Bytes()), func(t *pbt.T, b []byte) {
		t.Collect(toRange(50, len(b)))
	}))
}

type profile struct {
	name string
	age  int
	bio  string
}

var profileGen = pbt.Custom("profile", func(t *pbt.T) profile {
	return profile{
		name: pbt.Resize(10, pbt.String()).Draw(t, ""),
		age:  pbt.IntMin(1).Draw(t, ""),
		bio:  pbt.Sized(func(size int) pbt.Gen[string] { return pbt.Resize(size*35, pbt.String()) }).Draw(t, ""),
	}
})

func TestProfile(t *testing.T) {
	pbt.Check(t, pbt.ForAll(profileGen, func(t *pbt.T, p profile) {
		t.Aggregate("name "+toRange(10, len(p.name)), "bio "+toRange(300, len(p.bio)))
		if p.age < 1 {
			t.Fatalf("age %d", p.age)
		}
	}))
}

// Section 5.5: a generator that transforms another with Bind, the ?LET of
// the chapter, builds a queue from a list of pairs.
type pair struct{ key, val int }

type queue struct{ items []pair }

func fromList(ps []pair) queue { return queue{items: ps} }

var queueGen = pbt.Bind(pbt.SliceOf(pbt.Custom("pair", func(t *pbt.T) pair {
	return pair{pbt.Int().Draw(t, ""), pbt.Int().Draw(t, "")}
})), func(ps []pair) pbt.Gen[queue] { return pbt.Just(fromList(ps)) })

func TestQueue(t *testing.T) {
	pbt.Check(t, pbt.ForAll(queueGen, func(t *pbt.T, q queue) {
		if q.items == nil && len(q.items) != 0 {
			t.Fatalf("not a queue")
		}
	}))
}

// Section 5.6: restricting with Filter (?SUCHTHAT) retries; transforming
// with Map (?LET) never discards. Both pairs generate the same values.
var (
	evenFiltered   = pbt.Filter(pbt.Int(), func(n int) bool { return n%2 == 0 })
	unevenFiltered = pbt.Filter(pbt.Int(), func(n int) bool { return n%2 != 0 })
	evenMapped     = pbt.Map(pbt.IntRange(-1<<30, 1<<30), func(n int) int { return n * 2 })
	unevenMapped   = pbt.Map(pbt.IntRange(-1<<30, 1<<30), func(n int) int { return n*2 + 1 })
)

func TestEvenAndUneven(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		if e := evenFiltered.Draw(t, "e1"); e%2 != 0 {
			t.Fatalf("%d", e)
		}
		if e := evenMapped.Draw(t, "e2"); e%2 != 0 {
			t.Fatalf("%d", e)
		}
		if u := unevenFiltered.Draw(t, "u1"); u%2 == 0 {
			t.Fatalf("%d", u)
		}
		if u := unevenMapped.Draw(t, "u2"); u%2 == 0 {
			t.Fatalf("%d", u)
		}
	})
}

// Section 5.6: printable Latin-1 strings, first by filtering, then by
// construction, which the chapter recommends for narrow filters.
func printableLatin1(s string) bool {
	for _, r := range s {
		if r > 0xFF || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

var latin1Printable = func() []rune {
	var rs []rune
	for r := rune(0); r <= 0xFF; r++ {
		if unicode.IsPrint(r) {
			rs = append(rs, r)
		}
	}
	return rs
}()

func TestLatin1Strings(t *testing.T) {
	pbt.Check(t, pbt.ForAll(pbt.Filter(pbt.StringN(0, 5), printableLatin1), func(t *pbt.T, s string) {
		if !printableLatin1(s) {
			t.Fatalf("%q", s)
		}
	}))
	pbt.Check(t, pbt.ForAll(pbt.StringOf(pbt.SampledFrom(latin1Printable)), func(t *pbt.T, s string) {
		if !printableLatin1(s) {
			t.Fatalf("%q", s)
		}
	}))
}

// Section 5.7: a probabilistic generator for text that looks like prose,
// weighting letters over spaces over punctuation.
var textLike = pbt.StringOf(pbt.Frequency(
	pbt.Weighted[rune]{Weight: 80, Gen: pbt.SampledFrom([]rune("abcdefghijklmnopqrstuvwxyz"))},
	pbt.Weighted[rune]{Weight: 10, Gen: pbt.Just(' ')},
	pbt.Weighted[rune]{Weight: 1, Gen: pbt.Just('\n')},
	pbt.Weighted[rune]{Weight: 1, Gen: pbt.SampledFrom([]rune(".,;:!?"))},
	pbt.Weighted[rune]{Weight: 1, Gen: pbt.SampledFrom([]rune("0123456789"))},
))

func TestTextLike(t *testing.T) {
	pbt.Check(t, pbt.ForAll(textLike, func(t *pbt.T, s string) {
		t.Classify(strings.Contains(s, " "), "has spaces")
		t.Classify(strings.ContainsAny(s, ".,;:!?"), "has punctuation")
	}))
}

// Section 5.9: the chapter builds dictionaries through symbolic calls so
// that a failing case shows how the value was built rather than its
// internal structure. pbt reports the labeled draws, which serves the same
// purpose: the report below shows the map literal and the inputs, not the
// hash table.
func TestDictionaryReportIsReadable(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		d := pbt.MapOf(pbt.IntRange(0, 100), pbt.Bool()).Draw(t, "dict")
		if len(d) >= 3 {
			t.Fatalf("size %d", len(d))
		}
	}, pbt.Seed(1))
	if !strings.Contains(report, "dict (MapOf(IntRange(0, 100), Bool())) = map[int]bool{0:false, 1:false, 2:false}") {
		t.Errorf("report does not show the minimal map:\n%s", report)
	}
}

// Exercise 4: the probabilistic tree generator of the chapter could grow
// forever. Bounding it with the size parameter, halved for each child,
// makes it terminate with trees of an interesting size.
type tree struct {
	left, right *tree
	value       int
}

func depth(n *tree) int {
	if n == nil || n.left == nil {
		return 0
	}
	return 1 + max(depth(n.left), depth(n.right))
}

func boundedTree(size int) pbt.Gen[*tree] {
	leaf := pbt.Map(pbt.Int(), func(v int) *tree { return &tree{value: v} })
	if size <= 1 {
		return leaf
	}
	child := pbt.Lazy(func() pbt.Gen[*tree] { return boundedTree(size / 2) })
	return pbt.Frequency(
		pbt.Weighted[*tree]{Weight: 1, Gen: leaf},
		pbt.Weighted[*tree]{Weight: 3, Gen: pbt.Custom("branch", func(t *pbt.T) *tree {
			return &tree{left: child.Draw(t, ""), right: child.Draw(t, "")}
		})},
	)
}

func TestBoundedTree(t *testing.T) {
	pbt.Check(t, pbt.ForAll(pbt.Sized(boundedTree), func(t *pbt.T, n *tree) {
		d := depth(n)
		t.Collect(d)
		if d > 7 {
			t.Fatalf("depth %d exceeds log2 of the maximum size", d)
		}
	}))
}

// Exercise 5: three pairs of timestamp generators, each written once with a
// filter and once with a transformation. The transformations never discard;
// the filter reads better when the excluded range is small.
type stamp struct{ hour, minute int }

var (
	anyStamp = pbt.Custom("stamp", func(t *pbt.T) stamp {
		return stamp{pbt.IntRange(0, 23).Draw(t, ""), pbt.IntRange(0, 59).Draw(t, "")}
	})
	amStampFiltered = pbt.Filter(anyStamp, func(s stamp) bool { return s.hour < 12 })
	amStampMapped   = pbt.Map(anyStamp, func(s stamp) stamp { return stamp{s.hour % 12, s.minute} })

	stampPair = pbt.Custom("stamps", func(t *pbt.T) [2]stamp {
		return [2]stamp{anyStamp.Draw(t, ""), anyStamp.Draw(t, "")}
	})
	orderedFiltered = pbt.Filter(stampPair, func(p [2]stamp) bool { return !after(p[0], p[1]) })
	orderedMapped   = pbt.Map(stampPair, func(p [2]stamp) [2]stamp {
		if after(p[0], p[1]) {
			return [2]stamp{p[1], p[0]}
		}
		return p
	})

	inStandup         = func(s stamp) bool { return s.hour == 10 && s.minute < 30 }
	noStandupFiltered = pbt.Filter(anyStamp, func(s stamp) bool { return !inStandup(s) })
	noStandupMapped   = pbt.Map(anyStamp, func(s stamp) stamp {
		if inStandup(s) {
			return stamp{s.hour, s.minute + 30}
		}
		return s
	})
)

func after(a, b stamp) bool { return a.hour > b.hour || (a.hour == b.hour && a.minute > b.minute) }

func TestTimestampGenerators(t *testing.T) {
	for _, g := range []pbt.Gen[stamp]{amStampFiltered, amStampMapped} {
		pbt.Check(t, pbt.ForAll(g, func(t *pbt.T, s stamp) {
			if s.hour >= 12 {
				t.Fatalf("%v is not in the morning", s)
			}
		}))
	}
	for _, g := range []pbt.Gen[[2]stamp]{orderedFiltered, orderedMapped} {
		pbt.Check(t, pbt.ForAll(g, func(t *pbt.T, p [2]stamp) {
			if after(p[0], p[1]) {
				t.Fatalf("%v is not ordered", p)
			}
		}))
	}
	for _, g := range []pbt.Gen[stamp]{noStandupFiltered, noStandupMapped} {
		pbt.Check(t, pbt.ForAll(g, func(t *pbt.T, s stamp) {
			if inStandup(s) {
				t.Fatalf("%v is during standup", s)
			}
		}))
	}
}

// Exercise 6: a generator that creates a file and returns its handle. The
// chapter uses symbolic calls so that a failure shows what was written
// rather than the handle; here the labeled draw of the lines does that.
var fileGen = pbt.Custom("file", func(t *pbt.T) *os.File {
	lines := pbt.SliceOf(pbt.BytesN(1, 10)).Draw(t, "lines")
	f, err := os.CreateTemp(t.Name()+"_tmp", "pbt-*")
	if err != nil {
		f, err = os.CreateTemp("", "pbt-*")
	}
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	t.Cleanup(func() { f.Close(); os.Remove(f.Name()) })
	for _, l := range lines {
		if _, err := f.Write(l); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	return f
})

func TestFileGenerator(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		f := fileGen.Draw(t, "f")
		info, err := f.Stat()
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Size() >= 3 {
			t.Fatalf("file has %d bytes", info.Size())
		}
	}, pbt.Seed(1))
	if !strings.Contains(report, "  lines (SliceOf(BytesN(1, 10))) = [][]uint8{") {
		t.Errorf("report does not show the written lines:\n%s", report)
	}
}
