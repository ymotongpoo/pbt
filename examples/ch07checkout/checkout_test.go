// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch07checkout

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// The specification's example: A is 50 or 3 for 130, B is 30 or 2 for 45,
// C is 20 and D is 15, in any order.
func TestSpecificationExample(t *testing.T) {
	prices := []Price{{"A", 50}, {"B", 30}, {"C", 20}, {"D", 15}}
	specials := []Special{{"A", 3, 130}, {"B", 2, 45}}
	got, err := Total(strings.Split("B,A,B,A,A,C", ","), prices, specials)
	if err != nil || got != 130+45+20 {
		t.Errorf("Total = %d, %v; want 195", got, err)
	}
}

// Section 7.3: the generators are layered so that the expected price is
// computed independently of the implementation: first the price list, then
// the items picked from it with their running total.
type priced struct {
	items    []string
	expected int
	prices   []Price
	specials []Special
}

var priceList = pbt.SliceOfNDistinct(pbt.Custom("price", func(t *pbt.T) Price {
	return Price{pbt.StringN(1, 3).Draw(t, ""), pbt.IntRange(-1000, 1000).Draw(t, "")}
}), 1, -1, func(p Price) string { return p.Item })

func itemList(prices []Price) pbt.Gen[priced] {
	return pbt.Map(pbt.SliceOf(pbt.SampledFrom(prices)), func(picked []Price) priced {
		p := priced{prices: prices}
		for _, pr := range picked {
			p.items = append(p.items, pr.Item)
			p.expected += pr.Unit
		}
		return p
	})
}

var itemPriceList = pbt.Bind(priceList, itemList)

func TestNoSpecials(t *testing.T) {
	pbt.Check(t, pbt.ForAll(itemPriceList, func(t *pbt.T, p priced) {
		t.Collect(bucket(len(p.items)))
		got, err := Total(p.items, p.prices, nil)
		if err != nil || got != p.expected {
			t.Fatalf("Total(%v) = %d, %v; want %d", p.items, got, err, p.expected)
		}
	}))
}

func bucket(n int) string {
	lo := n / 10 * 10
	return fmt.Sprintf("%d-%d", lo, lo+10)
}

// Section 7.5: specials. Regular items are generated in quantities below
// the special threshold and special items in multiples of it, so that the
// two expected totals add up without overlap.
func specialList(prices []Price) pbt.Gen[[]Special] {
	return pbt.SliceOfDistinct(pbt.Custom("special", func(t *pbt.T) Special {
		return Special{pbt.SampledFrom(prices).Draw(t, "").Item, pbt.IntRange(2, 5).Draw(t, ""), pbt.IntRange(-1000, 1000).Draw(t, "")}
	}), func(s Special) string { return s.Item })
}

type part struct {
	items    []string
	expected int
}

func regularGen(prices []Price, specials []Special) pbt.Gen[part] {
	return pbt.Custom("regular", func(t *pbt.T) part {
		var p part
		for _, pr := range prices {
			var n int
			if sp, ok := specialFor(pr.Item, specials); ok {
				n = pbt.IntRange(0, sp.Count-1).Draw(t, "")
			} else {
				n = pbt.IntRange(0, 20).Draw(t, "")
			}
			for range n {
				p.items = append(p.items, pr.Item)
			}
			p.expected += n * pr.Unit
		}
		return p
	})
}

func specialGen(prices []Price, specials []Special) pbt.Gen[part] {
	return pbt.Custom("special", func(t *pbt.T) part {
		var p part
		for _, sp := range specials {
			multiplier := pbt.IntRange(0, 5).Draw(t, "")
			for range sp.Count * multiplier {
				p.items = append(p.items, sp.Item)
			}
			p.expected += multiplier * sp.Price
		}
		return p
	})
}

func specialFor(item string, specials []Special) (Special, bool) {
	for _, s := range specials {
		if s.Item == item {
			return s, true
		}
	}
	return Special{}, false
}

// itemPriceSpecial merges the regular and the special parts. The second
// variant, the solution of exercise 4 of Chapter 8, uses LetShrink so that
// a failure can shrink to one of the two parts on its own.
func itemPriceSpecial(shrinkable bool) pbt.Gen[priced] {
	return pbt.Bind(priceList, func(prices []Price) pbt.Gen[priced] {
		return pbt.Bind(specialList(prices), func(specials []Special) pbt.Gen[priced] {
			parts := []pbt.Gen[part]{regularGen(prices, specials), specialGen(prices, specials)}
			combine := func(ps []part) part {
				var out part
				for _, p := range ps {
					out.items = append(out.items, p.items...)
					out.expected += p.expected
				}
				return out
			}
			var merged pbt.Gen[part]
			if shrinkable {
				merged = pbt.LetShrink(parts, combine)
			} else {
				merged = pbt.Custom("merged", func(t *pbt.T) part {
					return combine([]part{parts[0].Draw(t, ""), parts[1].Draw(t, "")})
				})
			}
			return pbt.Map(merged, func(p part) priced {
				return priced{items: p.items, expected: p.expected, prices: prices, specials: specials}
			})
		})
	})
}

func TestSpecials(t *testing.T) {
	for _, shrinkable := range []bool{false, true} {
		pbt.Check(t, pbt.ForAll(itemPriceSpecial(shrinkable), func(t *pbt.T, p priced) {
			items := pbt.Permutation(p.items).Draw(t, "order")
			got, err := Total(items, p.prices, p.specials)
			if err != nil || got != p.expected {
				t.Fatalf("Total(%v, %v, %v) = %d, %v; want %d", items, p.prices, p.specials, got, err, p.expected)
			}
		}))
	}
}

// Section 7.7: negative tests. With arbitrary lists, Total must either
// return a price or one of its documented errors, never panic. The hybrid
// generator mixes known items with arbitrary ones so that the known ones
// overlap with the price list often enough; the chapter found that 92% of
// its first runs had no overlap at all.
var knownItems = []string{"A", "B", "C"}

var laxItems = pbt.SliceOf(pbt.OneOf(pbt.SampledFrom(knownItems), pbt.StringN(0, 3)))

var laxPrices = pbt.SliceOf(pbt.Custom("price", func(t *pbt.T) Price {
	return Price{pbt.OneOf(pbt.SampledFrom(knownItems), pbt.StringN(0, 3)).Draw(t, ""), pbt.IntRange(-1000, 1000).Draw(t, "")}
}))

var laxSpecials = pbt.SliceOf(pbt.Custom("special", func(t *pbt.T) Special {
	return Special{pbt.OneOf(pbt.SampledFrom(knownItems), pbt.StringN(0, 3)).Draw(t, ""), pbt.IntRange(0, 5).Draw(t, ""), pbt.IntRange(-1000, 1000).Draw(t, "")}
}))

func TestExpectedResult(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		items := laxItems.Draw(t, "items")
		prices := laxPrices.Draw(t, "prices")
		specials := laxSpecials.Draw(t, "specials")
		_, err := Total(items, prices, specials)
		switch {
		case err == nil:
			t.Classify(true, "priced")
		case errors.Is(err, ErrUnknownItem):
			t.Classify(true, "unknown item")
		case errors.Is(err, ErrInvalidPriceList), errors.Is(err, ErrInvalidSpecialList):
			t.Classify(true, "invalid list")
		default:
			t.Fatalf("unexpected error %v", err)
		}
	}, pbt.CheckCoverage())
}

// A list of n+1 picks from n items has a duplicate, so these lists must be
// rejected.
func TestDuplicatePricesAreInvalid(t *testing.T) {
	dupes := pbt.SliceOfN(pbt.SampledFrom(knownItems), len(knownItems)+1, len(knownItems)+1)
	pbt.Check(t, pbt.ForAll(dupes, func(t *pbt.T, items []string) {
		prices := make([]Price, len(items))
		for i, it := range items {
			prices[i] = Price{it, 1}
		}
		if _, err := Total(nil, prices, nil); !errors.Is(err, ErrInvalidPriceList) {
			t.Fatalf("Total accepted the duplicate price list %v: %v", prices, err)
		}
	}))
	pbt.Check(t, pbt.ForAll(dupes, func(t *pbt.T, items []string) {
		specials := make([]Special, len(items))
		for i, it := range items {
			specials[i] = Special{it, 2, 1}
		}
		prices := []Price{{"A", 1}, {"B", 1}, {"C", 1}}
		if _, err := Total(nil, prices, specials); !errors.Is(err, ErrInvalidSpecialList) {
			t.Fatalf("Total accepted the duplicate special list %v: %v", specials, err)
		}
	}))
	if _, err := Total(nil, []Price{{"A", 1}}, []Special{{"A", 0, 1}}); !errors.Is(err, ErrInvalidSpecialList) {
		t.Errorf("a special with count 0 was accepted: %v", err)
	}
	if !slices.Contains(knownItems, "A") {
		t.Fatal("A must be a known item")
	}
}
