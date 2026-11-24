// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package format generates strings in formats that standards define:
//
//   - dates, times and durations of RFC 3339, ISO 8601, RFC 9110 and
//     RFC 5322, and the layouts of package time;
//   - IP addresses in the text forms of RFC 4291 and RFC 5952, CIDR
//     prefixes, MAC addresses, host names of RFC 1123 and RFC 1035, and
//     internationalized host names with the Punycode of RFC 3492;
//   - e-mail addresses of RFC 5321, RFC 5322, RFC 6531 and the HTML
//     standard;
//   - URIs and URI references of RFC 3986 and http URLs;
//   - HTTP tokens, field names and values, media types and cookies of
//     RFC 9110, RFC 6838 and RFC 6265;
//   - UUIDs of RFC 9562, JSON texts of RFC 8259 and RFC 7493, JSON
//     Pointers of RFC 6901, CSV files of RFC 4180, the encodings of
//     RFC 4648, Semantic Versioning 2.0.0 and the language tags of BCP 47;
//   - telephone numbers of ITU-T E.164, payment card numbers with the Luhn
//     check digit of ISO/IEC 7812, and ISBNs of ISO 2108.
//
// Every generator follows one production of its standard, and the name
// says which. A function named after a standard, such as RFC3339 or
// RFC5322AddrSpec, generates the alternatives of the production, which is
// what a parser of that standard must accept; the only parts left out are
// those the standard itself forbids generators to produce, such as the
// obsolete syntax of RFC 5322 §4, and each function's documentation names
// them, along with the bounds it puts on lengths the grammar leaves open.
// A function with a plain name, such as Email or HTTPDate, generates the
// common or canonical form, which is what a program should write. Time
// takes the layout constants of package time, so format.Time(time.RFC1123)
// generates what time.Time.Format writes for that layout.
//
// The strings are built from their grammar rather than filtered, so every
// value is valid, including the values the shrinker tries, and values
// shrink toward a short valid string, such as "a@a" for an e-mail address.
// The shrinker prefers fewer choices, so it may switch a part to another
// alternative, such as an unquoted cookie value for a quoted one, or keep
// the alternative it started from when switching would take more choices;
// each function's documentation lists the targets of every alternative. Where a parser of
// the standard library does not accept all of a production, the
// generator's documentation says how they differ; those differences are
// what property tests of a parser are for.
//
// The package is built on the generators of package pbt and has no other
// dependencies.
package format
