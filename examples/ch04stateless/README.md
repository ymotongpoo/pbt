# Chapter 3 and 4: what a property is, stateless properties

The code of both chapters is in this package: `Biggest` with its model, the
sort invariants, `Last`, and the solutions of exercises 3 to 7. The
exercises that ask for an explanation are answered here.

**Question 1.** The three strategies the chapter names for finding
properties: modeling (compare against a simpler implementation, as
`TestBiggest` does with the sorted list), rewording the specification or
generalizing example tests (as `TestLast` does by appending a known
element), and symmetric properties (encode then decode, as the CSV codec of
chapter 6 does).

**Question 2.** The call that returns a sample of a generator's data is
`gen.Example()` in pbt, the equivalent of `proper_gen:pick/1`. It takes an
optional seed, so the same call returns the same value when the seed
repeats, provided the generator code and pbt version are unchanged.

Chapter 2 of the book is about tooling and has no code. Its commands map
to Go as follows: `rebar3 proper` is `go test`, `rebar3 proper -n 10000` is
`go test -pbt.checks=10000`, `rebar3 proper -p prop_sort` is
`go test -run TestSort`, and `rebar3 as test shell` followed by
`proper_gen:pick/1` is a short `TestMain`-free experiment with
`gen.Example()` inside any test.
