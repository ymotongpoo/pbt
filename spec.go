// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"reflect"
)

// NoPanic builds a property that calls f with arguments generated from its
// parameter types, through Make with the given options, and fails when f
// panics. It is what PropEr's check_spec becomes when the only
// specification is a Go signature. Every argument appears in the report.
func NoPanic(f any, opts ...MakeOption) Property {
	fv := reflect.ValueOf(f)
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		panic(fmt.Sprintf("pbt: NoPanic needs a function, got %T", f))
	}
	cfg := &makeConfig{types: map[reflect.Type]Gen[any]{}, fields: map[reflect.Type]map[string]Gen[any]{}}
	for _, o := range opts {
		o(cfg)
	}
	args := make([]Gen[any], ft.NumIn())
	for i := range args {
		typ := ft.In(i)
		draw := makeDraw(typ, cfg, tag{}, nil)
		args[i] = Gen[any]{name: typ.String(), draw: func(t *T) any {
			return draw(t, &budget{left: max(t.size, 1)}).Interface()
		}}
	}
	return func(t *T) {
		in := make([]reflect.Value, len(args))
		for i, g := range args {
			in[i] = reflect.ValueOf(g.Draw(t, fmt.Sprintf("arg%d", i)))
			if !in[i].IsValid() {
				in[i] = reflect.Zero(ft.In(i))
			}
		}
		if rec := Recover(func() {
			if ft.IsVariadic() {
				fv.CallSlice(in)
			} else {
				fv.Call(in)
			}
		}); rec != nil {
			t.Fatalf("panic: %v", rec)
		}
	}
}
