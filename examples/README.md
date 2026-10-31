# Examples: the PropEr Testing book in Go

Each package ports the examples and exercises of one chapter of
*PropEr Testing* by Fred Hebert (the edition at
[propertesting.com](https://propertesting.com/)) to `go.ymotongpoo.com/pbt`.
Tests that demonstrate how pbt catches a planted bug run the property against
a recording test handle (`internal/expect`) and assert on the report, so the
suite stays green.

| Chapter | Package | Contents |
| --- | --- | --- |
| 1 | `ch01cash` | The cash register and its two properties |
| 3, 4 | `ch04stateless` | biggest, sort invariants, last, and exercises 3 to 7 |
| 5 | `ch05generators` | Collect, Aggregate, Resize, Sized, Bind, Filter and Map, Lazy recursion, readable reports, and exercises 4 to 6 |
| 6 | `ch06birthday` | The birthday greetings kata: CSV codec, employees, the birthday filter, the mail template |
| 7 | `ch07checkout` | The checkout kata with specials and the negative tests |
| 8 | `ch08shrinking` | ShrinkTo for date-times, LetShrink for trees and concatenations, the meal exercise |
| 9 | `ch09cache` | The bounded cache as a state machine, sequential and parallel, with a planted race |
| 10 | `ch10bookstore` | The bookstore inventory on SQLite as a state machine, with the shim intents in the command names |
| 11 | `ch11breaker` | The circuit breaker as a finite state machine on package fsm |

Run everything with `go test ./...` from this directory.
