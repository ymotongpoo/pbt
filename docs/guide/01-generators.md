# Generators
<!-- summary: The built-in generators, writing your own with Custom, composing them with Map, Bind, Filter, OneOf and Frequency, reading the draws in a report, and sampling outside a property. -->

A generator is a value of type `pbt.Gen[V]`. It knows how to produce a `V` from the choices that pbt records for the current case, and that record is what makes shrinking and replay work. You never build the record yourself: you combine generators and draw from them.

## The built-in generators

| Values | Generators |
| --- | --- |
| Integers | `Int`, `IntRange`, `IntMin`, `IntMax`, `IntRangeToward`; `Int32`, `Int64`, `Uint`, `Uint32`, `Uint64`, `Byte` and their `Range` forms; `Integer[I]` and `IntegerRange[I]` for any integer type |
| Floats | `Float64`, `Float64Range`, `Float32`, `Float32Range` for finite values; `AnyFloat64` adds NaN and the infinities |
| Text | `Bool`, `Rune`, `String`, `StringN`, `StringOf`, `StringOfN`, `Bytes`, `BytesN` |
| Collections | `SliceOf`, `SliceOfN`, `SliceOfDistinct`, `SliceOfNDistinct`, `MapOf`, `MapOfN`, `Permutation` |
| Choice | `Just`, `SampledFrom`, `OneOf`, `Frequency`, `Ptr` |
| Functions | `Func`, `Func2`, which generate pure functions for properties over callbacks |
| Anything | `Any`, a bounded mix of common types, and `Make[T]`, which derives a generator from a type (chapter 9) |

Integers lean toward the boundaries of their range and toward small values, because those are where bugs live. Lengths of strings and collections follow the size parameter of the case. A maximum length of `-1` means no bound.

## Building your own

`pbt.Custom` turns a function of `*pbt.T` into a generator. Inside it you draw from other generators, in any order and under any control flow.

<!-- snippet: generators_test.go#user -->

The name given to `Custom` is how the generator appears in reports. The labels of the draws inside it matter too: a labeled draw inside a generator is listed in the report, indented under the value it helped to build. Unlabeled draws, with `""`, are not listed, which is right for a value that is not interesting on its own.

## Composing generators

Three functions cover most compositions.

<!-- snippet: generators_test.go#compose -->

- `Map` applies a function to each value. The result shrinks as the input does.
- `Filter` discards the values that fail a predicate and draws again. A predicate that rejects most values makes the run slow, and pbt gives up on a case after many rejections. Prefer building the value you want: `IntRange(1, 100)` over `Filter(Int(), positive)`.
- `Bind` chooses the next generator from a value, which is how one part of a value can depend on another. PropEr calls it `?LET`.

`OneOf` picks one of several generators with equal probability and `Frequency` with weights; chapter 2 covers them. `Ptr` wraps a value in a pointer, with or without `nil`. `Lazy` defers the construction of a generator until it is first drawn from, which is what a recursive generator needs to refer to itself; chapter 9 shows the bounded form.

## Reports show how a value was built

A property that fails on a `User` reports the user and, under it, the labeled parts.

<!-- snippet: generators_test.go#user-report -->

```text
[pbt] property failed after 3 passed checks and 145 shrink steps (size 4)
[pbt] user (user) = snippets.User{Name:"aaaaaaaaa", Age:0, Email:"aaaaaaaaa@example.com"}
[pbt]   name (StringOfN(SampledFrom(26 values), 1, 12)) = "aaaaaaaaa"
[pbt]   age (IntRange(0, 120)) = 0
[pbt] error: display name "Aaaaaaaaa" is too long for the badge
[pbt] reproduce: go test -run 'TestDisplayName' -pbt.seed=0x1
```

The name shrank to the shortest string that still breaks the length check, nine letters of `a`, and the age to 0 because it played no part in the failure. A report lists labeled draws up to three levels deep and cuts each value at 2000 characters; the `ReportDepth` and `ReportLimit` options change both.

## Sampling outside a property

`Example` returns one value of a generator, for a quick look or for a conventional test that needs a plausible input. A seed makes the value stable.

<!-- snippet: generators_test.go#example -->

## Values that generators share

`Just(v)` returns the same `v` for every case. When `v` is a slice, a map or a pointer, every case sees the same object, so a property that modifies it changes what later cases and the report see. `SampledFrom(values)` copies the slice but hands out the elements as they are. Draw a fresh value from `Custom` when the property needs to modify it.
