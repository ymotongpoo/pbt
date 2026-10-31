package pbt

import (
	"fmt"
	"strings"
)

// Fn is a generated function from A to B, the equivalent of PropEr's
// function/2. The result for an argument is drawn from the test case the
// first time that argument is seen and cached, so the function is pure within
// the case, shrinks with it, and its observed calls appear in the report.
type Fn[A comparable, B any] struct {
	t     *T
	ret   Gen[B]
	table map[A]B
	order []A
}

// Call applies the function.
func (f *Fn[A, B]) Call(a A) B {
	if b, ok := f.table[a]; ok {
		return b
	}
	b := f.ret.drawSpan(f.t)
	f.table[a] = b
	f.order = append(f.order, a)
	return b
}

// String lists the calls observed so far in order of first call.
func (f *Fn[A, B]) String() string {
	parts := make([]string, len(f.order))
	for i, a := range f.order {
		parts[i] = fmt.Sprintf("%#v -> %#v", a, f.table[a])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// GoString makes %#v print the observed calls instead of the struct.
func (f *Fn[A, B]) GoString() string { return f.String() }

// Func generates functions of one argument whose results come from ret.
func Func[A comparable, B any](ret Gen[B]) Gen[*Fn[A, B]] {
	var zero A
	return Gen[*Fn[A, B]]{name: fmt.Sprintf("Func[%T](%s)", zero, ret.name), draw: func(t *T) *Fn[A, B] {
		return &Fn[A, B]{t: t, ret: ret, table: map[A]B{}}
	}}
}

type pair2[A, B comparable] struct {
	a A
	b B
}

// Fn2 is a generated function of two arguments; see Fn.
type Fn2[A, B comparable, C any] struct {
	f *Fn[pair2[A, B], C]
}

// Call applies the function.
func (f *Fn2[A, B, C]) Call(a A, b B) C { return f.f.Call(pair2[A, B]{a, b}) }

// String lists the calls observed so far in order of first call.
func (f *Fn2[A, B, C]) String() string {
	parts := make([]string, len(f.f.order))
	for i, k := range f.f.order {
		parts[i] = fmt.Sprintf("%#v, %#v -> %#v", k.a, k.b, f.f.table[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// GoString makes %#v print the observed calls instead of the struct.
func (f *Fn2[A, B, C]) GoString() string { return f.String() }

// Func2 generates functions of two arguments whose results come from ret.
func Func2[A, B comparable, C any](ret Gen[C]) Gen[*Fn2[A, B, C]] {
	var za A
	var zb B
	inner := Func[pair2[A, B]](ret)
	return Gen[*Fn2[A, B, C]]{name: fmt.Sprintf("Func2[%T, %T](%s)", za, zb, ret.name), draw: func(t *T) *Fn2[A, B, C] {
		return &Fn2[A, B, C]{f: inner.draw(t)}
	}}
}
