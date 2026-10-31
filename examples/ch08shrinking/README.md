# Chapter 8: shrinking

`shrinking_test.go` ports the date-time generator, the two tree generators
and the exercises with code. Questions 1 and 2 ask for explanations.

**Question 1.** The two macros are `?SHRINK(Gen, Alternatives)` and
`?LETSHRINK(Vars, Gens, Expr)`. The first generates from `Gen` and lets the
value shrink to values of the alternative generators, in order; in pbt it
is `pbt.ShrinkTo(g, alts...)`, with `pbt.IntRangeToward` as the common
case of pulling a number toward one value. The second generates every part
and combines them, and lets the combined value shrink to any single part;
in pbt it is `pbt.LetShrink(parts, combine)`.

**Question 2.** `?LETSHRINK` differs from `?LET` only during shrinking: a
`?LET` value shrinks by shrinking what it was built from and rebuilding,
whereas a `?LETSHRINK` value can also be replaced by one of its parts
without the combining expression. That is what lets a tree shrink to a
subtree, and what the exercise with the four-course meal needs.

A note on the engine: pbt shrinks the sequence of choices a generator
made rather than the values, so a value built with `pbt.Bind` already
shrinks through the bind. `LetShrink` records one extra choice, "combine
or return part N", which generation always leaves at "combine" and
shrinking can lower to a part.
