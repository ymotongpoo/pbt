# Standard formats
<!-- summary: The package format: strings in the formats that RFCs, ISO and other standards define, the naming rule that tells the whole grammar from the common form, Go's layout constants, and finding where a parser departs from its standard. -->

Many inputs of a program are strings in a format that a standard defines: timestamps, e-mail addresses, URIs, JSON, UUIDs, language tags. The package `go.ymotongpoo.com/pbt/format` generates them. Every generator returns a `pbt.Gen[string]` and builds its values from the grammar of the standard. Every value is valid, including the values the shrinker tries.

```go
import "go.ymotongpoo.com/pbt/format"
```

The generators of chapter 1 return Go types, such as a `time.Time` or a `netip.Addr`. The generators of this package return text. `pbt.IPv6()` gives an address; `format.RFC4291IPv6()` gives the ways to write one.

## Two names for two questions

A format raises two questions. What should a program write? What must a parser accept? The answers differ, and the package gives each one its own name.

- A function with a plain name, such as `Email`, `URL` or `HTTPDate`, generates the common or canonical form. Use it for the input of code that produces or stores the format, and for values that should pass every validator.
- A function named after a standard, such as `RFC5321Mailbox`, `RFC3986URI` or `RFC9110HTTPDate`, generates the alternatives of the standard's production. Use it to test a parser. It leaves out only what the standard forbids generators to produce, such as the obsolete syntax of RFC 5322. Its documentation names those parts and the bounds it puts on open lengths.

`Time(layout)` takes the layout constants of package `time`, so `format.Time(time.RFC1123)` generates what `time.Time.Format` writes for that layout. It is the alias for what Go writes; `RFC3339` is the alias for what RFC 3339 allows.

| Area | What a program writes | What a parser must accept |
| --- | --- | --- |
| Timestamps | `Time(time.RFC3339)`, `HTTPDate` | `RFC3339`, `RFC3339FullDate`, `RFC3339FullTime`, `RFC9110HTTPDate`, `RFC5322DateTime` |
| Durations | `ISO8601Duration` | `RFC3339Duration` |
| Addresses | `IPv4`, `RFC5952IPv6`, `CIDR`, `MAC` | `RFC4291IPv6` |
| Host names | `Hostname`, `IDNAHostname`, `UnicodeHostname` | `RFC1035Domain` |
| E-mail | `Email`, `HTMLEmail`, `SMTPUTF8Mailbox` | `RFC5321Mailbox`, `RFC5322AddrSpec` |
| URIs | `URL` | `RFC3986URI`, `RFC3986Reference` |
| HTTP | `HTTPToken`, `HTTPFieldName`, `HTTPFieldValue`, `MediaType` | `RFC9110FieldValue`, `RFC9110MediaType`, `RFC6265Cookie`, `RFC6265SetCookie` |
| Data | `JSON`, `IJSON`, `JSONPointer`, `JSONPointerFragment`, `UUID`, `SemVer`, `LanguageTag` | `RFC4180CSV`, `RFC5646LanguageTag` |
| Encodings | `Base64`, `Base64URL`, `Base32`, `Base32Hex`, `Base16`, `Hex` and the unpadded `Raw` forms | |
| Numbers | `PhoneNumber`, `PaymentCard`, `ISBN13`, `ISBN10` | |

## Testing a parser

A property that feeds a parser every form of its standard finds where the parser departs from it. Every RFC 3339 timestamp should parse with the layout of the same name.

<!-- snippet: formats_test.go#parser -->

```text
[pbt] property failed after 1 passed checks and 19 shrink steps (size 2)
[pbt] s (RFC3339()) = "1970-01-01T00:00:00z"
[pbt] error: time.Parse rejects 1970-01-01T00:00:00z: parsing time "1970-01-01T00:00:00z" as "2006-01-02T15:04:05Z07:00": cannot parse "z" as "Z07:00"
[pbt] reproduce: go test -run 'TestRFC3339' -pbt.seed=0x1
```

RFC 3339 allows a lower-case `z` and `t`, and `time.Parse` rejects them, as it rejects the leap second `23:59:60`. Whether that matters depends on where your timestamps come from. The property makes the decision visible, and the shrunk case shows the difference in one character. The documentation of each generator lists the differences between the standard and the parsers of Go's standard library, as measured on Go 1.26, and names the releases where a difference changed: `url.Parse` accepts IPvFuture literals such as `http://[v1.x]/` up to Go 1.25.1 and rejects them from Go 1.25.2. The tests of the package pin the differences on every supported version of Go.

## Testing what a program writes

For the input of a program, the plain names give values that every reasonable consumer accepts. A sign-up form can store `format.Email()` and expect `net/mail` to read the same address back.

<!-- snippet: formats_test.go#producer -->

The generators lean toward the cases that break code, as the generators of chapter 1 do. In a share of the cases:

- `Email` makes local parts of exactly 64 characters, and `Hostname` makes names of exactly 253.
- `RFC3339` writes leap seconds and the offset `-00:00`.
- `JSON` writes numbers far beyond 64 bits and unpaired surrogate escapes.

The documentation of each generator says how often.

<!-- snippet: formats_test.go#roundtrip -->

## How the values shrink

Values shrink toward short valid strings: `a@a.aa` for `Email`, `http://a` for `URL`, `1970-01-01T00:00:00Z` for `RFC3339`, `null` for `JSON`. The shrinker prefers fewer choices, so a part can also shrink within the alternative it started from. A quoted local part, for example, shrinks toward the empty quoted string, `""@a`. The documentation of each generator lists the targets of every alternative.

## What the package leaves out

The package generates syntax, not registered values.

- A language tag is well-formed, but its subtags are not checked against the IANA registry.
- A payment card number has a correct check digit, but no issuer assigned it.
- An ISBN is not hyphenated, since the hyphens depend on ranges that the ISBN agency publishes.

The package has no tables that follow a registry, such as the country codes of ISO 3166 or the currencies of ISO 4217, because they change with the registry. Write them with `SampledFrom` over the values your program supports.
