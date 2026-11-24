// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

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
// generated. Fields may carry a `pbt` tag: "-" leaves the field
// zero, and "min=A,max=B" bounds a number or the length of a string, slice
// or map. Interfaces, channels and functions need a generator supplied with
// WithType or WithField; Make panics otherwise. Unexported fields stay
// zero. Standard types time.Time, *time.Location, netip.Addr, netip.Prefix,
// netip.AddrPort, net.IP and net.HardwareAddr use the corresponding generators;
// locations use FixedZone without tzdata. Time bounds use RFC 3339 timestamps.
// Other types with unexported state need WithType; cmd/pbtgen can fill
// unexported fields of types in the package it writes into.
//
// One draw shares 8*max(size, 1) growth units across the whole value. Each
// collection element (including a string rune or a map candidate) and each
// non-nil pointer spends one unit. After exhaustion, pointers are nil and
// collections stop growing. Fixed fields, arrays and nonrecursive minimum
// lengths are completed; recursive collections may stop below their minimum
// to terminate. Recursive collections and pointers also stop after 64 nested
// growth steps, independently of the engine's span nesting limit.
// For a fixed type and tags, this bounds work and choices by
// C*(8*max(size, 1)+1), where C is the cost of completing its fixed fields,
// arrays and required nonrecursive elements, including up to ten attempts
// per distinct map key. WithType and WithField overrides are exempt, as are
// the standard-library generators above. Values that do not exhaust the
// budget or reach a depth limit keep their seeded choices. Values formerly
// truncated by the old recursion limits can change even below this budget.
func Make[V any](opts ...MakeOption) Gen[V] {
	cfg := &makeConfig{types: map[reflect.Type]Gen[any]{}, fields: map[reflect.Type]map[string]Gen[any]{}}
	for _, o := range opts {
		o(cfg)
	}
	typ := reflect.TypeFor[V]()
	for st, fields := range cfg.fields {
		for name := range fields {
			if st.Kind() != reflect.Struct {
				panic(fmt.Sprintf("pbt: Make: WithField on %s, which is not a struct", st))
			}
			if _, ok := st.FieldByName(name); !ok {
				panic(fmt.Sprintf("pbt: Make: WithField names %s.%s, which does not exist", st, name))
			}
		}
	}
	draw := makeDraw(typ, cfg, tag{}, nil)
	return BoundedGrowth(Gen[V]{name: fmt.Sprintf("Make[%s]", typ), draw: func(t *T) V {
		v, _ := draw(t, t.makeBudget).Interface().(V) // a nil interface value stays the zero V
		return v
	}})
}

// valueAs converts a value from a supplied generator to the type Make needs,
// treating a nil interface value as the zero value of that type.
func valueAs(v any, typ reflect.Type) reflect.Value {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return reflect.Zero(typ)
	}
	return rv.Convert(typ)
}

// tag is a parsed pbt struct tag. The bounds stay as text and are parsed
// at the width of the field they apply to, so that an int64 bound keeps
// every digit and a bound outside the type is rejected.
type tag struct {
	skip         bool
	min, max     string
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
		if !ok || v == "" {
			tg.parseFailure = part
			return tg
		}
		switch k {
		case "min":
			tg.min = v
		case "max":
			tg.max = v
		default:
			tg.parseFailure = part
			return tg
		}
	}
	return tg
}

type drawFn func(*T, *budget) reflect.Value

// makeDraw builds the drawing function for typ. stack holds the types being
// built, so that a recursive reference becomes a lazy draw sharing the growth budget.
func makeDraw(typ reflect.Type, cfg *makeConfig, tg tag, stack []reflect.Type) drawFn {
	if g, ok := cfg.types[typ]; ok {
		return func(t *T, _ *budget) reflect.Value { return valueAs(UnboundedGrowth(g).drawSpan(t), typ) }
	}
	if draw := standardDraw(typ, tg); draw != nil {
		return draw
	}
	for _, s := range stack {
		if s == typ {
			// The recursive reference is resolved once, after the whole
			// type has been built, and shared safely between goroutines.
			var once sync.Once
			var inner drawFn
			return func(t *T, b *budget) reflect.Value {
				once.Do(func() { inner = makeDraw(typ, cfg, tg, nil) })
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
		lo, hi := floatBounds(typ, tg)
		return func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(drawFloat(t, lo, hi)).Convert(typ) }
	case reflect.String:
		minLen, maxLen := lengthBounds(tg)
		g := StringOfN(Rune(), minLen, maxLen)
		return func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(g.drawSpan(t)).Convert(typ) }
	case reflect.Slice:
		minLen, maxLen := lengthBounds(tg)
		elem := makeDraw(typ.Elem(), cfg, tag{}, stack)
		return func(t *T, b *budget) reflect.Value {
			g := Gen[reflect.Value]{name: typ.Elem().String(), draw: func(t *T) reflect.Value { return elem(t, b) }}
			vals := drawSlice(t, g, minLen, maxLen, nil, recursiveType(typ))
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
			pair := Gen[[2]reflect.Value]{name: "entry", draw: func(t *T) [2]reflect.Value { return [2]reflect.Value{key(t, b), val(t, b)} }}
			drawSlice(t, pair, minLen, maxLen, func(kv [2]reflect.Value) bool {
				if out.MapIndex(kv[0]).IsValid() {
					return false
				}
				out.SetMapIndex(kv[0], kv[1])
				return true
			}, recursiveType(typ))
			return out
		}
	case reflect.Pointer:
		elem := makeDraw(typ.Elem(), cfg, tag{}, stack)
		return func(t *T, b *budget) reflect.Value {
			if !t.drawBool(0.8) || b.left <= 0 || b.depth >= maxGrowthDepth {
				return reflect.Zero(typ)
			}
			b.enter()
			defer b.leave()
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
				fields = append(fields, field{i, func(t *T, _ *budget) reflect.Value { return valueAs(UnboundedGrowth(g).drawSpan(t), ft) }})
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
	if tg.min != "" {
		lo = parseBound(tg.min, func(s string) (int64, error) { return strconv.ParseInt(s, 10, bits) })
	}
	if tg.max != "" {
		hi = parseBound(tg.max, func(s string) (int64, error) { return strconv.ParseInt(s, 10, bits) })
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
	if tg.min != "" {
		lo = parseBound(tg.min, func(s string) (uint64, error) { return strconv.ParseUint(s, 10, bits) })
	}
	if tg.max != "" {
		hi = parseBound(tg.max, func(s string) (uint64, error) { return strconv.ParseUint(s, 10, bits) })
	}
	if lo > hi {
		panic(fmt.Sprintf("pbt: Make: min %d is above max %d", lo, hi))
	}
	return lo, hi
}

func floatBounds(typ reflect.Type, tg tag) (lo, hi float64) {
	lo, hi = -math.MaxFloat64, math.MaxFloat64
	bits := 64
	if typ.Kind() == reflect.Float32 {
		lo, hi, bits = -math.MaxFloat32, math.MaxFloat32, 32
	}
	if tg.min != "" {
		lo = parseBound(tg.min, func(s string) (float64, error) { return strconv.ParseFloat(s, bits) })
	}
	if tg.max != "" {
		hi = parseBound(tg.max, func(s string) (float64, error) { return strconv.ParseFloat(s, bits) })
	}
	if lo > hi {
		panic(fmt.Sprintf("pbt: Make: min %v is above max %v", lo, hi))
	}
	return lo, hi
}

func lengthBounds(tg tag) (minLen, maxLen int) {
	minLen, maxLen = 0, -1
	if tg.min != "" {
		minLen = parseBound(tg.min, func(s string) (int, error) { return strconv.Atoi(s) })
	}
	if tg.max != "" {
		maxLen = parseBound(tg.max, func(s string) (int, error) { return strconv.Atoi(s) })
	}
	if minLen < 0 || (maxLen >= 0 && maxLen < minLen) {
		panic(fmt.Sprintf("pbt: Make: invalid length bounds %d, %d", minLen, maxLen))
	}
	return minLen, maxLen
}

// parseBound parses a tag bound at the width of its field and panics on a
// value the field cannot hold, instead of rounding or wrapping it.
func parseBound[N any](s string, parse func(string) (N, error)) N {
	v, err := parse(s)
	if err != nil {
		panic(fmt.Sprintf("pbt: Make: bound %q does not fit the field: %v", s, err))
	}
	return v
}
