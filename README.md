# pbt

Property-based testing for Go, aiming at the feature set of Erlang's
[PropEr](https://proper-testing.github.io/): typed generators with
automatic shrinking, user-controlled distributions with statistics to verify
them, stateful and parallel command testing with symbolic variables, and
targeted properties.

**Status: under development.** The API is not stable yet.

```go
import "go.ymotongpoo.com/pbt"
```

Requires Go 1.25 or later.

The design rationale, the comparison with rapid, PropEr and QuickCheck, and
the implementation plan are in [DESIGN.md](DESIGN.md) (Japanese).

## License

[Apache License 2.0](LICENSE.md)
