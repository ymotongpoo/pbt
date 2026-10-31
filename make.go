// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// maxMakeDepth bounds how deep Make follows pointers and collections of a
// recursive type before it stops with nil and empty values.
const maxMakeDepth = 40

// budget counts the recursive nodes one top-level draw may still create.
// Without it, a type whose fields hold slices of itself grows exponentially
// with the size parameter; with it, a value holds about size such nodes.
type budget struct{ left int }

// MakeOption adjusts the generator that Make derives from a type.
type MakeOption func(*makeConfig)

type makeConfig struct {
	types  map[reflect.Type]Gen[any]
	fields map[reflect.Type]map[string]Gen[any]
}

// WithType makes Make use g for every value of type V, including fields,
// elements and the top-level value.
func WithType[V any](g Gen[V]) MakeOption {
	return func(c *makeConfig) {
		c.types[reflect.TypeFor[V]()] = asAny(g)
	}
}

// WithField makes Make use g for the field of struct type S.
func WithField[S, V any](field string, g Gen[V]) MakeOption {
	return func(c *makeConfig) {
		typ := reflect.TypeFor[S]()
		if c.fields[typ] == nil {
			c.fields[typ] = map[string]Gen[any]{}
		}
		c.fields[typ][field] = asAny(g)
	}
}

func asAny[V any](g Gen[V]) Gen[any] {
	return Gen[any]{name: g.name, draw: func(t *T) any { return g.drawSpan(t) }}
}

// Make derives a generator for V from its type, the way PropEr derives
// generators from type declarations. Booleans, integers, floats and strings
// use the generators of this package; slices, arrays, maps and pointers
// are built from their element types; structs get every exported field
// generated, and recursion through pointers and collections ends after
// maxMakeDepth levels. Fields may carry a `pbt` tag: "-" leaves the field
// zero, and "min=A,max=B" bounds a number or the length of a string, slice
// or map. Interfaces, channels and functions need a generator supplied with
// WithType or WithField; Make panics otherwise.
func Make[V any](opts ...MakeOption) Gen[V] {
	cfg := &makeConfig{types: map[reflect.Type]Gen[any]{}, fields: map[reflect.Type]map[string]Gen[any]{}}
	for _, o := range opts {
		o(cfg)
	}
	typ := reflect.TypeFor[V]()
	draw := makeDraw(typ, cfg, tag{}, nil)
	return Gen[V]{name: fmt.Sprintf("Make[%s]", typ), draw: func(t *T) V {
		return draw(t, &budget{left: max(t.size, 1)}).Interface().(V)
	}}
}

type tag struct {
	skip         bool
	hasMin       bool
	hasMax       bool
	minV, maxV   float64
	parseFailure string
}

func parseTag(s string) tag {
	var tg tag
	if s == "-" {
		tg.skip = true
		return tg
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		f, err := strconv.ParseFloat(v, 64)
		if !ok || err != nil {
			tg.parseFailure = part
			return tg
		}
		switch k {
		case "min":
			tg.hasMin, tg.minV = true, f
		case "max":
			tg.hasMax, tg.maxV = true, f
		default:
			tg.parseFailure = part
			return tg
		}
	}
	return tg
}

type drawFn func(*T, *budget) reflect.Value

// makeDraw builds the drawing function for typ. stack holds the types being
// built, so that a recursive reference becomes a lazy, depth-bounded draw.
func makeDraw(typ reflect.Type, cfg *makeConfig, tg tag, stack []reflect.Type) drawFn {
	if g, ok := cfg.types[typ]; ok {
		return func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(g.drawSpan(t)).Convert(typ) }
	}
	for _, s := range stack {
		if s == typ {
			var inner drawFn
			return func(t *T, b *budget) reflect.Value {
				if b.left <= 0 || len(t.open) > maxMakeDepth {
					return reflect.Zero(typ)
				}
				b.left--
				if inner == nil {
					inner = makeDraw(typ, cfg, tg, nil)
				}
				return inner(t, b)
			}
		}
	}
	stack = append(stack, typ)
	switch typ.Kind() {
	case reflect.Bool:
		return func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(t.drawBool(0.5)).Convert(typ) }
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		lo, hi := intBounds(typ, tg)
		meta := zigzagMeta(min(max(lo, 0), hi), lo, hi)
		c, width := min(max(lo, 0), hi), uint64(hi)-uint64(lo)
		return func(t *T, _ *budget) reflect.Value {
			return reflect.ValueOf(zigzagDecode(t.draw(width, meta, modeBiased), c, lo, hi)).Convert(typ)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		lo, hi := uintBounds(typ, tg)
		return func(t *T, _ *budget) reflect.Value {
			return reflect.ValueOf(lo + t.draw(hi-lo, plainMeta, modeBiased)).Convert(typ)
		}
	case reflect.Float32, reflect.Float64:
		lo, hi := -math.MaxFloat64, math.MaxFloat64
		if typ.Kind() == reflect.Float32 {
			lo, hi = -math.MaxFloat32, math.MaxFloat32
		}
		if tg.hasMin {
			lo = tg.minV
		}
		if tg.hasMax {
			hi = tg.maxV
		}
		return func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(drawFloat(t, lo, hi)).Convert(typ) }
	case reflect.String:
		minLen, maxLen := lengthBounds(tg)
		g := StringOfN(Rune(), minLen, maxLen)
		return func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(g.drawSpan(t)).Convert(typ) }
	case reflect.Slice:
		minLen, maxLen := lengthBounds(tg)
		elem := makeDraw(typ.Elem(), cfg, tag{}, stack)
		return func(t *T, b *budget) reflect.Value {
			if len(t.open) > maxMakeDepth {
				return reflect.MakeSlice(typ, 0, 0)
			}
			g := Gen[reflect.Value]{name: typ.Elem().String(), draw: func(t *T) reflect.Value { return elem(t, b) }}
			vals := drawSlice(t, g, minLen, maxLen, nil)
			out := reflect.MakeSlice(typ, len(vals), len(vals))
			for i, v := range vals {
				out.Index(i).Set(v)
			}
			return out
		}
	case reflect.Array:
		elem := makeDraw(typ.Elem(), cfg, tag{}, stack)
		return func(t *T, b *budget) reflect.Value {
			out := reflect.New(typ).Elem()
			for i := range typ.Len() {
				out.Index(i).Set(elem(t, b))
			}
			return out
		}
	case reflect.Map:
		minLen, maxLen := lengthBounds(tg)
		key := makeDraw(typ.Key(), cfg, tag{}, stack)
		val := makeDraw(typ.Elem(), cfg, tag{}, stack)
		return func(t *T, b *budget) reflect.Value {
			out := reflect.MakeMap(typ)
			if len(t.open) > maxMakeDepth {
				return out
			}
			pair := Gen[[2]reflect.Value]{name: "entry", draw: func(t *T) [2]reflect.Value { return [2]reflect.Value{key(t, b), val(t, b)} }}
			drawSlice(t, pair, minLen, maxLen, func(kv [2]reflect.Value) bool {
				if out.MapIndex(kv[0]).IsValid() {
					return false
				}
				out.SetMapIndex(kv[0], kv[1])
				return true
			})
			return out
		}
	case reflect.Pointer:
		elem := makeDraw(typ.Elem(), cfg, tag{}, stack)
		return func(t *T, b *budget) reflect.Value {
			if len(t.open) > maxMakeDepth || !t.drawBool(0.8) {
				return reflect.Zero(typ)
			}
			p := reflect.New(typ.Elem())
			p.Elem().Set(elem(t, b))
			return p
		}
	case reflect.Struct:
		type field struct {
			index int
			draw  drawFn
		}
		var fields []field
		for i := range typ.NumField() {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			ftag := parseTag(f.Tag.Get("pbt"))
			if ftag.parseFailure != "" {
				panic(fmt.Sprintf("pbt: Make: field %s.%s has an invalid pbt tag %q", typ, f.Name, ftag.parseFailure))
			}
			if ftag.skip {
				continue
			}
			if g, ok := cfg.fields[typ][f.Name]; ok {
				ft := f.Type
				fields = append(fields, field{i, func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(g.drawSpan(t)).Convert(ft) }})
				continue
			}
			fields = append(fields, field{i, makeDraw(f.Type, cfg, ftag, stack)})
		}
		return func(t *T, b *budget) reflect.Value {
			out := reflect.New(typ).Elem()
			for _, f := range fields {
				out.Field(f.index).Set(f.draw(t, b))
			}
			return out
		}
	default:
		panic(fmt.Sprintf("pbt: Make cannot derive a generator for %s; supply one with WithType or WithField", typ))
	}
}

func intBounds(typ reflect.Type, tg tag) (lo, hi int64) {
	bits := typ.Bits()
	lo, hi = -1<<(bits-1), 1<<(bits-1)-1
	if tg.hasMin {
		lo = int64(tg.minV)
	}
	if tg.hasMax {
		hi = int64(tg.maxV)
	}
	if lo > hi {
		panic(fmt.Sprintf("pbt: Make: min %d is above max %d", lo, hi))
	}
	return lo, hi
}

func uintBounds(typ reflect.Type, tg tag) (lo, hi uint64) {
	bits := typ.Bits()
	hi = math.MaxUint64
	if bits < 64 {
		hi = 1<<bits - 1
	}
	if tg.hasMin {
		lo = uint64(max(tg.minV, 0))
	}
	if tg.hasMax {
		hi = uint64(max(tg.maxV, 0))
	}
	if lo > hi {
		panic(fmt.Sprintf("pbt: Make: min %d is above max %d", lo, hi))
	}
	return lo, hi
}

func lengthBounds(tg tag) (minLen, maxLen int) {
	minLen, maxLen = 0, -1
	if tg.hasMin {
		minLen = int(max(tg.minV, 0))
	}
	if tg.hasMax {
		maxLen = int(max(tg.maxV, 0))
	}
	if maxLen >= 0 && maxLen < minLen {
		panic(fmt.Sprintf("pbt: Make: min length %d is above max length %d", minLen, maxLen))
	}
	return minLen, maxLen
}
