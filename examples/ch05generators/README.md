# Chapter 5: custom generators

The generators and properties of the chapter are in `generators_test.go`,
together with exercises 4 to 6. The first three exercises ask for
explanations.

**Question 1.** The functions that show the distribution of the generated
data are `t.Collect` and `t.Aggregate` (PropEr's `collect/2` and
`aggregate/2`), joined by `t.Classify` and `t.Measure`. `Collect` and
`Classify` count each recorded label and divide by the number of passing
cases; repeated labels within one case can produce shares above 100%.
`Aggregate` reports shares of all samples, and `Measure` reports a numeric
minimum, average and maximum. pbt prints these statistics in the test log
when the property passes; run `go test -v` to see them. `Cover`, unlike
`Collect` and `Classify`, counts each case once per label.

**Question 2.** The macro that modifies a generator's output with ordinary
code is `?LET`, which is `pbt.Bind` when the next generator depends on the
value and `pbt.Map` when a plain function is enough.

**Question 3.** `?LAZY` is needed because Erlang evaluates the arguments of
a function before the call, so a recursive generator would build itself
forever before generating anything. Go evaluates eagerly too, so a
generator that refers to itself must go through `pbt.Lazy`, which defers
the construction of the inner generator until the first draw. The bounded
tree of exercise 4 shows the second half of the answer: laziness alone does
not make the recursion terminate; the size parameter does.
