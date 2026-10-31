# pbt and rapid on the same properties

The `bench/` module runs the same generators and the same failing
properties under pbt and under [rapid](https://github.com/flyingmutant/rapid)
v1.3.0. Reproduce the numbers with:

```sh
cd bench && go test -bench . -run 'TestCompareShrinking|TestVolume' -v ./...
```

The figures below come from an Apple M4 with Go 1.27.1 on 2026-10-31. They
change from run to run; read the ratios, not the digits.

## Throughput

One iteration is a `Check` of 100 passing cases that draw one value.

| Generator | pbt, default sizes | pbt, `Sizes(1, 6)` | rapid |
| --- | --- | --- | --- |
| `SliceOf(Int())` | 242 µs | 107 µs | 61 µs |
| `SliceOf(String())` | 9.5 ms | 373 µs | 297 µs |
| `MapOf(String(), Int())` | 10.3 ms | | 348 µs |
| `Make[point]()` (two ints and a string slice) | 10.1 ms | | 346 µs |

The default sizes dominate the first column. pbt follows PropEr: the size
parameter grows from 1 to 42 over the run and collection lengths follow it,
while rapid keeps collections short by default. In 100 cases of
`SliceOf(String())` the two libraries generate very different amounts of
data:

| | strings | runes |
| --- | --- | --- |
| pbt, default sizes | 3,893 | 151,210 |
| pbt, `Sizes(1, 6)` | 553 | 3,283 |
| rapid | 522 | 2,563 |

Per element, pbt is on par or faster: about 63 ns per rune against 99 ns,
and 72 ns per int against 110 ns, while it records the span structure and
the choice metadata that its shrinker and its reports use. When a property
is slow per case, cap the size with `pbt.Sizes` or `-pbt.maxsize`.

## Shrinking

Each row runs one failing property under both libraries, from a random
seed, with 100 cases and the default shrink budget. Times are the whole
`Check`, including the search for the first failure; rapid's include
writing its failure file.

| Property | pbt minimum | pbt | rapid minimum | rapid |
| --- | --- | --- | --- | --- |
| int greater than a million | `1000001` | 0 ms | `1000001` | 4 ms |
| two elements above a million | `[1000001, 1000001]` | 2 ms | `[1000001, 1000001]` | 10 ms |
| length 1 mod 3 with a large last element | `[0, 0, 0, 100]` | 0 ms | `[0, 0, 0, 100]` | 3 ms |
| five distinct positive elements | `[1, 2, 3, 4, 5]` | 1 ms | `[1, 2, 3, 4, 5]` | 11 ms |
| first string longer than the second | `"0"`, `""` | 0 ms | `"A"`, `""` | 1 ms |
| struct with a tag and X above Y | `{X:1, Y:0, Tags:[""]}` | 0 ms | `{X:1, Y:0, Tags:[""]}` | 3 ms |
| map with three entries | three keys `""`, `"0"`, `"1"` | 3 ms | three keys `""`, `"A"`, `"a"` | 0 ms |

The minima agree except for the rune the two libraries consider simplest:
pbt shrinks runes along its table, which starts with the digits, and rapid
shrinks toward `A`.
