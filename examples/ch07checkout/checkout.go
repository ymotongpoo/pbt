// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ch07checkout ports the checkout kata of Chapter 7 of the PropEr
// Testing book: a total over scanned items with unit prices and bulk
// specials, with the validation that the negative tests of the chapter
// demanded.
package ch07checkout

import (
	"errors"
	"fmt"
)

// Price is the unit price of an item.
type Price struct {
	Item string
	Unit int
}

// Special is a bulk price: Count items for Price.
type Special struct {
	Item  string
	Count int
	Price int
}

var (
	// ErrUnknownItem reports an item without a unit price.
	ErrUnknownItem = errors.New("unknown item")
	// ErrInvalidPriceList reports duplicate entries in the price list.
	ErrInvalidPriceList = errors.New("invalid price list")
	// ErrInvalidSpecialList reports a special with a count of zero or
	// duplicate entries in the special list.
	ErrInvalidSpecialList = errors.New("invalid special list")
)

// Total prices the items: each item's count is covered by its special as
// many times as possible and the remainder is charged at the unit price.
// Items may come in any order.
func Total(items []string, prices []Price, specials []Special) (int, error) {
	if !validPriceList(prices) {
		return 0, ErrInvalidPriceList
	}
	if !validSpecialList(specials) {
		return 0, ErrInvalidSpecialList
	}
	counts := map[string]int{}
	for _, it := range items {
		counts[it]++
	}
	total := 0
	for _, sp := range specials {
		n := counts[sp.Item]
		total += (n / sp.Count) * sp.Price
		counts[sp.Item] = n % sp.Count
	}
	for item, n := range counts {
		unit, err := costOfItem(item, prices)
		if err != nil {
			return 0, err
		}
		total += n * unit
	}
	return total, nil
}

func costOfItem(item string, prices []Price) (int, error) {
	for _, p := range prices {
		if p.Item == item {
			return p.Unit, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", ErrUnknownItem, item)
}

func validPriceList(prices []Price) bool {
	seen := map[string]bool{}
	for _, p := range prices {
		if seen[p.Item] {
			return false
		}
		seen[p.Item] = true
	}
	return true
}

func validSpecialList(specials []Special) bool {
	seen := map[string]bool{}
	for _, s := range specials {
		if s.Count <= 0 || seen[s.Item] {
			return false
		}
		seen[s.Item] = true
	}
	return true
}
