// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"fmt"
	"strings"

	"go.ymotongpoo.com/pbt"
)

// Character sets of RFC 3986 §2.2, §2.3 and §3.3, the simplest first.
const (
	unreserved = alnum + "-._~"
	subDelims  = "!$&'()*+,;="
	pchar      = unreserved + subDelims + ":@"
)

var (
	nibbles  = pbt.Byte()
	hexUpper = choice(3, 1) // upper case, which §2.1 recommends, or lower case
	// pctEncoded generates a pct-encoded octet of §2.1, each hexadecimal
	// digit in either case.
	pctEncoded = pbt.Custom("pct-encoded", func(t *pbt.T) string {
		o := nibbles.Draw(t, "")
		hi, lo := hexUpper.Draw(t, "") == 0, hexUpper.Draw(t, "") == 0
		return string([]byte{'%', hexDigit(o>>4, hi), hexDigit(o, lo)})
	})
)

// pctString generates strings of characters of set and percent-encoded
// octets, at least minLen items and at most maxLen, one item in ten an
// octet. Each item sits in its own span, so the shrinker deletes items one
// by one.
func pctString(set string, minLen, maxLen int) pbt.Gen[string] {
	item := pbt.Frequency(pbt.W(9, char(set)), pbt.W(1, pctEncoded))
	return pbt.Map(pbt.SliceOfN(item, minLen, maxLen), func(items []string) string { return strings.Join(items, "") })
}

var (
	// The number of items is bounded to keep URIs at the sizes programs
	// meet; the grammar has no limits.
	segment     = pctString(pchar, 0, 16)
	segmentNZ   = pctString(pchar, 1, 16)
	segmentNZNC = pctString(unreserved+subDelims+"@", 1, 16)
	queryText   = pctString(pchar+"/?", 0, 32)
	userinfo    = pctString(unreserved+subDelims+":", 0, 16)
	regName     = pctString(unreserved+subDelims, 0, 32)

	// pathAbempty generates path-abempty: up to eight "/" segment.
	pathAbempty = pbt.Map(pbt.SliceOfN(segment, 0, 8), func(ss []string) string {
		if len(ss) == 0 {
			return ""
		}
		return "/" + strings.Join(ss, "/")
	})
	// pathAbsolute generates path-absolute: "/" and an optional non-empty
	// segment followed by path-abempty.
	pathAbsolute = concat(pbt.Just("/"), optional(1, 1, concat(segmentNZ, pathAbempty)))
	pathRootless = concat(segmentNZ, pathAbempty)
	pathNoscheme = concat(segmentNZNC, pathAbempty)

	// ipvFuture generates IPvFuture of §3.2.2 in brackets; the "v" is
	// case-insensitive like every ABNF string.
	ipvFuture = concat(pbt.Just("["), pbt.Frequency(pbt.W(3, pbt.Just("v")), pbt.W(1, pbt.Just("V"))),
		stringIn("0123456789abcdefABCDEF", 1, 4), pbt.Just("."), stringIn(unreserved+subDelims+":", 1, 16), pbt.Just("]"))
	host = pbt.Frequency(
		pbt.W(4, pbt.Custom("host name", func(t *pbt.T) string { return fitLabels(hostnameLabels.Draw(t, ""), 253) })),
		pbt.W(2, regName),
		pbt.W(2, IPv4()),
		pbt.W(2, concat(pbt.Just("["), ipv6Text("IPv6address", 1, false), pbt.Just("]"))),
		pbt.W(1, ipvFuture),
	)
	// authority generates authority of §3.2: an optional userinfo, a host
	// and an optional port of up to ten digits, none included.
	authority = concat(
		optional(4, 1, concat(userinfo, pbt.Just("@"))),
		host,
		optional(2, 1, concat(pbt.Just(":"), stringIn(digits, 0, 10))),
	)
	query    = optional(2, 1, concat(pbt.Just("?"), queryText))
	fragment = optional(2, 1, concat(pbt.Just("#"), queryText))

	scheme = pbt.Frequency(
		pbt.W(4, pbt.SampledFrom([]string{"http", "https", "ftp", "file", "ws", "wss", "mailto", "urn", "data", "tel"})),
		pbt.W(1, concat(char(alpha), stringIn(alnum+"+-.", 0, 8))),
	)
	hierPart = pbt.Frequency(
		pbt.W(5, concat(pbt.Just("//"), authority, pathAbempty)),
		pbt.W(2, pathAbsolute),
		pbt.W(2, pathRootless),
		pbt.W(1, pbt.Just("")),
	)
	relativePart = pbt.Frequency(
		pbt.W(3, concat(pbt.Just("//"), authority, pathAbempty)),
		pbt.W(2, pathAbsolute),
		pbt.W(2, pathNoscheme),
		pbt.W(1, pbt.Just("")),
	)
	uri         = concat(scheme, pbt.Just(":"), hierPart, query, fragment)
	relativeRef = concat(relativePart, query, fragment)
)

// RFC3986URI generates URIs in the syntax of RFC 3986 §3: a scheme, a
// common one such as "http" or any letter followed by letters, digits, "+",
// "-" and ".", then ":" and a hierarchical part that is "//" and an
// authority followed by an absolute path or none, an absolute path, a
// rootless path, or nothing, then an optional query and fragment. The
// authority has an optional userinfo and an optional port of up to ten
// digits, none included, and a host that is a host name, any registered
// name of unreserved, sub-delims and percent-encoded characters including
// the empty one, an IPv4 address, an IPv6 address in any form of RFC 4291
// in brackets, or an IPvFuture literal whose "v" is in either case.
// Paths, queries and fragments hold every character §3.3 and §3.4 allow,
// percent-encoded octets with hexadecimal digits in either case among
// them. Paths have up to nine segments of up to 16 items, a character or a
// percent-encoded octet, and queries and fragments up to 32 items, which
// the grammar does not limit. Values shrink toward "http://a"; a part
// whose other forms would take more choices keeps its form, so a URI
// without an authority can shrink toward "http:/", "http:a" or "http:", one
// with another kind of host toward "http://", "http://0.0.0.0",
// "http://[::]" or "http://[v0.a]", and one with another scheme toward
// "a://a".
//
// net/url.Parse rejects two kinds of these valid URIs: percent-encoded
// octets below 0x80 in a registered name, such as "http://%41/", where it
// accepts only %25 and octets of non-ASCII characters, and IPvFuture
// literals such as "http://[v1.x]/". It rejects IPvFuture literals since
// Go 1.25.2, which accepts only an IPv6 address in brackets; Go 1.25.0 and
// 1.25.1 accept them.
func RFC3986URI() pbt.Gen[string] { return named(uri, "RFC3986URI()") }

// RFC3986Reference generates URI references in the syntax of RFC 3986
// §4.1: in half of the cases a URI of RFC3986URI, and otherwise a relative
// reference of §4.2: a network-path reference that starts with "//" and an
// authority, an absolute-path reference, a relative-path reference whose
// first segment holds no colon, or an empty path, each with an optional
// query and fragment. Values shrink toward "http://a"; a relative
// reference can keep its form and shrink toward "//a", "/", "a" or the
// empty reference.
func RFC3986Reference() pbt.Gen[string] {
	return named(pbt.OneOf(uri, relativeRef), "RFC3986Reference()")
}

var (
	urlHost = pbt.Frequency(
		pbt.W(8, pbt.Custom("host name", func(t *pbt.T) string { return fitLabels(hostnameLabels.Draw(t, ""), 253) })),
		pbt.W(1, IPv4()),
		pbt.W(1, concat(pbt.Just("["), RFC5952IPv6(), pbt.Just("]"))),
	)
	urlPort     = optional(2, 1, pbt.Map(pbt.IntRange(1, 65535), func(p int) string { return fmt.Sprintf(":%d", p) }))
	urlSegments = pbt.Map(pbt.SliceOfN(pctString(unreserved+"!$&'()*+,;=:@", 0, 12), 0, 4), func(ss []string) string {
		if len(ss) == 0 {
			return ""
		}
		return "/" + strings.Join(ss, "/")
	})
	urlValue  = pctString(unreserved+"!$'()*+,;:@/?", 0, 12)
	urlParams = pbt.Map(pbt.SliceOfN(concat(pctString(unreserved, 1, 8), pbt.Just("="), urlValue), 0, 4), func(ps []string) string {
		if len(ps) == 0 {
			return ""
		}
		return "?" + strings.Join(ps, "&")
	})
	httpURL = concat(
		pbt.SampledFrom([]string{"http://", "https://"}),
		urlHost, urlPort, urlSegments, urlParams,
		optional(2, 1, concat(pbt.Just("#"), urlValue)),
	)
)

// URL generates absolute http and https URLs, the subset of RFC 3986 that
// web clients and servers exchange: the scheme in lower case, "://", a host
// that is a host name of Hostname, an IPv4 address, or an IPv6 address in
// the form of RFC 5952 in brackets, an optional port from 1 to 65535, a
// path of up to four segments of unreserved characters, sub-delims and
// percent-encoded octets, an optional query of up to four key=value pairs
// joined by "&", and an optional fragment. url.Parse reads every value
// with the scheme and host it was built from. Values shrink toward
// "http://a".
func URL() pbt.Gen[string] { return named(httpURL, "URL()") }
