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

Run everything with `go test ./...` from this directory.
