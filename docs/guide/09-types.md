# Generators from types

Writing a generator for every struct of a program is tedious, and most of the time the type already says what the values look like. pbt derives generators from types in two ways: `Make[T]` at run time through reflection, and `cmd/pbtgen` at development time as Go source.

## Make

<!-- snippet: types_test.go#make -->

`Make[T]` builds a generator for `T` from its fields, recursively. A `pbt` struct tag adjusts a field: `min` and `max` bound a number or the length of a string, slice or map, and `-` leaves the field zero. Two options replace generators: `WithType(g)` for every value of a type, which is how `time.Time` gets a generator above, and `WithField[S](name, g)` for one field of a struct.

Some types need help. Unexported fields stay zero, because reflection cannot set them, so a type whose state is all unexported needs `WithType`. Interfaces, channels and functions have no derivable generator and need `WithType` or `WithField` as well; `Make` panics with the field name when one is missing.

`NoPanic` from chapter 4 uses `Make` for the arguments of the function it tests, and accepts the same options.

## Recursive types

A type that contains itself has no finite generator unless something bounds the recursion. `Recursive` gives a leaf generator and a function that extends a generator by one level; the size parameter bounds how many levels a value gets, so a value has about `size` nodes.

<!-- snippet: types_test.go#recursive -->

`Bounded(leaf, node)` is the lower-level form: it keeps one recursion budget per value, shared by every bounded generator drawn while that value is built, which is what two mutually recursive types need. `Make` applies the same budget to recursive structs, so a self-referential struct does not grow without end.

## pbtgen

`cmd/pbtgen` writes the generators as source, which the compiler checks and which can set unexported fields because the generated file lives in the package itself.

```text
//go:generate go run go.ymotongpoo.com/pbt/cmd/pbtgen -type Order,Customer
```

For each type `T`, `go generate` writes a function `GenT` returning `pbt.Gen[T]` into `pbt_gen.go`. The struct tags of `Make` apply, and a `gen=EXPR` tag names a Go expression to use as the generator of a field, such as a variable of type `pbt.Gen[string]` in the same package. Recursive types, including mutually recursive ones, are generated through `Bounded`.

Three flags adjust the output. `-output` names the file. `-prefix` replaces `Gen` in the function names, and pbtgen refuses to write a name that the package already declares, so a hand-written generator is never overwritten. pbtgen reads the files that `go build` would compile on the current platform, so files excluded by build constraints do not affect the generated code.

Choose `Make` when the type changes often or the generator is needed in one test; choose pbtgen when the generator is part of the package's test support, when unexported fields matter, or when a compile error is preferable to a panic at test time.
