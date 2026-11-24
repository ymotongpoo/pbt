// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package format generates strings in formats that standards define:
// timestamps and durations of RFC 3339, ISO 8601, RFC 9110 and RFC 5322,
// IP addresses in the text forms of RFC 4291 and RFC 5952, MAC addresses,
// host names of RFC 1123 and RFC 1035, e-mail addresses of RFC 5321,
// RFC 5322 and the HTML standard, URIs of RFC 3986, UUIDs of RFC 9562 and
// JSON texts of RFC 8259 and RFC 7493.
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
// The shrinker keeps the form of each part it starts from, so a quoted
// local part shrinks toward the empty quoted string rather than toward an
// atom; each function's documentation lists the targets. Where a parser of
// the standard library does not accept all of a production, the
// generator's documentation says how they differ; those differences are
// what property tests of a parser are for.
//
// The package is built on the generators of package pbt and has no other
// dependencies.
package format
