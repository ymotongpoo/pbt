// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"reflect"
	"sync"
)

// maxGrowthDepth leaves room for generated wrappers below maxSpanDepth.
const maxGrowthDepth = 64

// budget counts growth steps, shared by every collection and pointer in a
// derived value. Fixed fields and required nonrecursive elements are finished
// even after exhaustion, so that number and length tags remain valid.
type budget struct{ left, depth int }

func (b *budget) enter() {
	if b != nil {
		b.spend()
		b.depth++
	}
}
func (b *budget) leave() {
	if b != nil {
		b.depth--
	}
}

func (b *budget) spend() {
	if b != nil && b.left > 0 {
		b.left--
	}
}

// BoundedGrowth limits optional growth in g with the budget described by Make:
// 8*max(size, 1) collection elements, string runes, map candidates or non-nil
// pointers, shared across nested draws. Recursive collections and nullable
// pointers also stop after 64 nested growth steps, before the engine's nesting
// limit. Fixed fields and required nonrecursive elements are still completed.
//
// This helper supports the code cmd/pbtgen writes. A hand-written generator
// can wrap a Custom generator that combines SliceOf, MapOf, String and Ptr
// to share the same allowance instead of giving every collection its full
// size. Nested BoundedGrowth calls share the outer allowance; independent
// top-level draws get fresh budgets. It does not limit arbitrary work in
// Custom callbacks or non-nullable pointers. Use UnboundedGrowth for a field
// whose generator must retain its own contract, such as an explicit override.
// Outside BoundedGrowth these collection and pointer generators are unchanged.
func BoundedGrowth[V any](g Gen[V]) Gen[V] {
	return Gen[V]{name: g.name, draw: func(t *T) V {
		if t.makeBudget != nil {
			return g.draw(t)
		}
		// Avoid overflow even for a size near MaxInt.
		n := min(max(t.size, 1), int(^uint(0)>>1)/8) * 8
		t.makeBudget = &budget{left: n}
		defer func() { t.makeBudget = nil }()
		return g.draw(t)
	}}
}

// UnboundedGrowth runs g without the surrounding BoundedGrowth allowance,
// then restores that allowance, including when g discards or panics. With no
// surrounding allowance it has no effect on generation. A BoundedGrowth call
// inside g starts a separate allowance.
//
// This helper supports the code cmd/pbtgen writes for explicit gen tags and
// standard-library generators. Hand-written generators can use it for fields
// whose own size or validity constraints take precedence over the surrounding
// growth limit. Work performed by g is excluded from that limit.
func UnboundedGrowth[V any](g Gen[V]) Gen[V] {
	return Gen[V]{name: g.name, draw: func(t *T) V {
		saved := t.makeBudget
		t.makeBudget = nil
		defer func() { t.makeBudget = saved }()
		return g.draw(t)
	}}
}

var recursiveTypes sync.Map // reflect.Type -> bool

// recursiveType ignores fields that Make skips. A cycle through a collection
// needs a termination exception to its minimum length; acyclic minima can
// always be completed with a fixed amount of work per growth step.
func recursiveType(typ reflect.Type) bool {
	if v, ok := recursiveTypes.Load(typ); ok {
		return v.(bool)
	}
	active := map[reflect.Type]bool{}
	seen := map[reflect.Type]bool{}
	var visit func(reflect.Type) bool
	visit = func(t reflect.Type) bool {
		if active[t] {
			return true
		}
		if seen[t] {
			return false
		}
		seen[t], active[t] = true, true
		defer delete(active, t)
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			return visit(t.Elem())
		case reflect.Map:
			return visit(t.Key()) || visit(t.Elem())
		case reflect.Struct:
			for i := range t.NumField() {
				f := t.Field(i)
				if f.Tag.Get("pbt") != "-" && visit(f.Type) {
					return true
				}
			}
		}
		return false
	}
	v := visit(typ)
	recursiveTypes.Store(typ, v)
	return v
}
