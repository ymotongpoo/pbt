# Generators
<!-- summary: The built-in generators, including times, network addresses, Unicode classes and regular expressions, writing your own with Custom, composing them with Map, Bind, Filter, OneOf and Frequency, reading the draws in a report, and sampling outside a property. -->

A generator is a value of type `pbt.Gen[V]`. It knows how to produce a `V` from the choices that pbt records for the current case, and that record is what makes shrinking and replay work. You never build the record yourself: you combine generators and draw from them.

## The built-in generators

| Values | Generators |
| --- | --- |
| Integers | `Int`, `IntRange`, `IntMin`, `IntMax`, `IntRangeToward`; `Int32`, `Int64`, `Uint`, `Uint32`, `Uint64`, `Byte` and their `Range` forms; `Integer[I]` and `IntegerRange[I]` for any integer type |
| Floats | `Float64`, `Float64Range`, `Float32`, `Float32Range` for finite values; `AnyFloat64` adds NaN and the infinities |
| Text | `Bool`, `Rune`, `RuneIn`, `String`, `StringN`, `StringOf`, `StringOfN`, `StringMatching`, `Bytes`, `BytesN` |
| Time | `Time`, `TimeRange`, `Duration`, `DurationRange`, `FixedZone`, `Zone` |
| Network | `IPv4`, `IPv6`, `Addr`, `AddrIn`, `Prefix`, `AddrPort` for `net/netip`, and `HardwareAddr` |
| Collections | `SliceOf`, `SliceOfN`, `SliceOfDistinct`, `SliceOfNDistinct`, `MapOf`, `MapOfN`, `Permutation` |
| Choice | `Just`, `SampledFrom`, `OneOf`, `Frequency`, `Ptr` |
| Functions | `Func`, `Func2`, which generate pure functions for properties over callbacks |
| Anything | `Any`, a bounded mix of common types, and `Make[T]`, which derives a generator from a type (chapter 9) |

Integers lean toward the boundaries of their range and toward small values, because those are where bugs live. Lengths of strings and collections follow the size parameter of the case. A maximum length of `-1` means no bound. Strings in the formats that standards define, such as RFC 3339 timestamps, e-mail addresses and URIs, come from the package `go.ymotongpoo.com/pbt/format`, which chapter 10 describes.

## Times and addresses

The generators of times and addresses follow the same rule as the integers: they lean toward the values where code breaks. `Time` draws an instant between the zero `time.Time` and the end of year 9999. It moves nearly half of the instants to an edge of the calendar: the first or last nanosecond of a day, a month or a year, February 29, or a bound of the range. In a share of the cases, it also rounds the fraction of a second to whole microseconds, milliseconds or seconds, since stored timestamps mostly carry those. A date one month ahead is a classic victim.

<!-- snippet: standard_test.go#billing -->

```text
[pbt] property failed after 39 passed checks and 13 shrink steps (size 40)
[pbt] start (Time()) = time.Date(1970, time.January, 31, 23, 59, 59, 999999999, time.UTC)
[pbt] error: next billing date 1970-03-03 is not in February
[pbt] reproduce: go test -run 'TestBilling' -pbt.seed=0x1
```

`AddDate` normalizes January 31 plus one month to March 3. The run found the last nanosecond of a month. The shrinker then moved it to January 1970, the month of the Unix epoch, which is where `Time` shrinks. `TimeRange(lo, hi)` keeps values in a range and in the location of `lo`. In a location with daylight saving time, it also moves values next to a change of offset: to the moment of the change, the nanosecond before it, or the half hours around it. `Zone` loads IANA time zones, by default a list of zones with unusual rules, and `FixedZone` draws fixed offsets.

The address generators return the types of `net/netip`. `IPv4` and `IPv6` draw a share of their addresses from the special-purpose blocks of RFC 6890. Among them are loopback, private networks, link local, documentation and multicast, and, for IPv6, IPv4-mapped addresses. `AddrIn` keeps addresses inside the prefixes you give it.

<!-- snippet: standard_test.go#network -->

## Strings that match a pattern

`StringMatching` builds strings from a regular expression in the syntax of package `regexp`. Every value matches the whole pattern, and shrinking keeps it matching. Alternations shrink toward earlier branches, repetitions toward their minimum count, and character classes toward their first printable ASCII member. `RuneIn` draws runes from the range tables of package `unicode`, such as scripts and categories. Every range of the tables gets the same chance, so a small script is not drowned out by the CJK ideographs.

<!-- snippet: standard_test.go#matching -->

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
[pbt] property failed after 3 passed checks and 189 shrink steps (size 4)
[pbt] user (user) = snippets.User{Name:"aaaaaaaaa", Age:0, Email:"aaaaaaaaa@example.com"}
[pbt]   name (StringOfN(SampledFrom(26 values), 1, 12)) = "aaaaaaaaa"
[pbt]   age (IntRange(0, 120)) = 0
[pbt] error: display name "Aaaaaaaaa" is too long for the badge
[pbt] reproduce: go test -run 'TestDisplayName' -pbt.seed=0x1
```

The name shrank to the shortest string that still breaks the length check, nine letters of `a`, and the age to 0 because it played no part in the failure. A report lists labeled draws up to three levels deep and cuts each value at 2000 characters; the `ReportDepth` and `ReportLimit` options change both.

A draw at the top level of the property appears in the report even without a label. When the draw is assigned to a single variable, as in `start := pbt.Time().Draw(t, "")` above, the report names the value after the variable. Otherwise it gives the file and the line of the draw, as in `Int() at prop_test.go:12 = 5`. The report reads the source file to find the variable. When the file is not available, for example in a binary built with `-trimpath`, the report gives the line. `Verbose()` or `-pbt.v` adds the line to every draw.

## Sampling outside a property

`Example` returns one value of a generator, for a quick look or for a conventional test that needs a plausible input. A seed makes the value stable for the same generator code and pbt version.

<!-- snippet: generators_test.go#example -->

## Values that generators share

`Just(v)` returns the same `v` for every case. When `v` is a slice, a map or a pointer, every case sees the same object, so a property that modifies it changes what later cases and the report see. `SampledFrom(values)` copies the slice but hands out the elements as they are. Draw a fresh value from `Custom` when the property needs to modify it.
