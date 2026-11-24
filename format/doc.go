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
// RFC5322AddrSpec, generates everything the production allows, which is
// what a parser of that standard must accept. A function with a plain
// name, such as Email or HTTPDate, generates the common or canonical form,
// which is what a program should write. Time takes the layout constants of
// package time, so format.Time(time.RFC1123) generates what
// time.Time.Format writes for that layout.
//
// The strings are built from their grammar rather than filtered, so every
// value is valid, including the values the shrinker tries, and values
// shrink toward the shortest valid string, such as "a@a" for an e-mail
// address. Where a parser of the standard library does not accept all of a
// production, the generator's documentation says how they differ; those
// differences are what property tests of a parser are for.
//
// The package is built on the generators of package pbt and has no other
// dependencies.
package format
