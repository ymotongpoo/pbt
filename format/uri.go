// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"fmt"
	"net/netip"
	"strings"

	"go.ymotongpoo.com/pbt"
)

// Character sets of RFC 3986 §2.2, §2.3 and §3.3, the simplest first.
const (
	unreserved = alnum + "-._~"
	subDelims  = "!$&'()*+,;="
	pchar      = unreserved + subDelims + ":@"
)

var pctItem = choice(9, 1) // a character, a percent-encoded octet

// pctString generates strings of characters of set and percent-encoded
// octets, at least minLen items and at most maxLen, negative for no limit.
// Each item sits in its own span, so the shrinker deletes items one by one.
func pctString(set string, minLen, maxLen int) pbt.Gen[string] {
	item := pbt.Custom("item", func(t *pbt.T) string {
		if pctItem.Draw(t, "") == 0 {
			return string(charIn(set).Draw(t, ""))
		}
		o := pbt.Byte().Draw(t, "")
		up := pbt.Frequency(pbt.W(3, pbt.Just(true)), pbt.W(1, pbt.Just(false))).Draw(t, "")
		return string([]byte{'%', hexDigit(o>>4, up), hexDigit(o, up)})
	})
	return pbt.Map(pbt.SliceOfN(item, minLen, maxLen), func(items []string) string { return strings.Join(items, "") })
}

var (
	// The lengths are bounded to keep URIs at the sizes programs meet;
	// the grammar has no limits.
	segment     = pctString(pchar, 0, 16)
	segmentNZ   = pctString(pchar, 1, 16)
	segmentNZNC = pctString(unreserved+subDelims+"@", 1, 16)
	queryText   = pctString(pchar+"/?", 0, 32)
	userinfo    = pctString(unreserved+subDelims+":", 0, 16)
	regName     = pctString(unreserved+subDelims, 0, 32)
	futureText  = stringIn(unreserved+subDelims+":", 1, 16)
	hexText     = stringIn("0123456789abcdefABCDEF", 1, 4)

	schemeForm    = choice(4, 1) // a common scheme, any scheme
	commonSchemes = pbt.SampledFrom([]string{"http", "https", "ftp", "file", "ws", "wss", "mailto", "urn", "data", "tel"})
	schemeRest    = stringIn(alnum+"+-.", 0, 8)
	hierForm      = choice(5, 2, 2, 1) // authority and path-abempty, path-absolute, path-rootless, path-empty
	relativeForm  = choice(3, 2, 2, 1) // authority and path-abempty, path-absolute, path-noscheme, path-empty
	hostForm      = choice(6, 2, 2, 1) // reg-name, IPv4address, IPv6address, IPvFuture
	regNameForm   = choice(3, 1)       // a host name, any reg-name
	optional      = choice(2, 1)       // absent, present
	portText      = stringIn(digits, 0, 6)
	segments      = pbt.SliceOfN(segment, 0, 8)
)

// RFC3986URI generates URIs in the syntax of RFC 3986 §3: a scheme, a
// common one such as "http" or any letter followed by letters, digits, "+",
// "-" and ".", then ":" and a hierarchical part that is "//" and an
// authority followed by an absolute path or none, an absolute path, a
// rootless path, or nothing, then an optional query and fragment. The
// authority has an optional userinfo and port, the port of any number of
// digits including none, and a host that is a registered name, often a
// host name and sometimes any unreserved, sub-delims and percent-encoded
// characters including the empty name, an IPv4 address, an IPv6 address in
// any form of RFC 4291 in brackets, or an IPvFuture literal. Paths,
// queries and fragments hold every character §3.3 and §3.4 allow,
// percent-encoded octets with upper- or lower-case digits among them.
// Paths have up to eight segments of up to 16 characters, and queries and
// fragments up to 32, which the grammar does not limit. Values shrink
// toward "http://a"; the shrinker keeps the form of the hierarchical part,
// so the other forms shrink toward "http:/", "http:a" and "http:".
//
// net/url.Parse rejects two kinds of these valid URIs: percent-encoded
// octets below 0x80 in a registered name, such as "http://%41/", where it
// accepts only %25 and octets of non-ASCII characters, and IPvFuture
// literals such as "http://[v1.x]/".
func RFC3986URI() pbt.Gen[string] {
	addrs6, addrs4 := pbt.IPv6(), pbt.IPv4()
	return pbt.Custom("RFC3986URI()", func(t *pbt.T) string {
		var b strings.Builder
		if schemeForm.Draw(t, "") == 0 {
			b.WriteString(commonSchemes.Draw(t, ""))
		} else {
			b.WriteByte(charIn(alpha).Draw(t, ""))
			b.WriteString(schemeRest.Draw(t, ""))
		}
		b.WriteByte(':')
		switch hierForm.Draw(t, "") {
		case 0:
			b.WriteString("//")
			writeAuthority(t, &b, addrs4, addrs6)
			writeSegments(t, &b)
		case 1:
			writePathAbsolute(t, &b)
		case 2:
			b.WriteString(segmentNZ.Draw(t, ""))
			writeSegments(t, &b)
		}
		writeQueryFragment(t, &b)
		return b.String()
	})
}

// RFC3986Reference generates URI references in the syntax of RFC 3986
// §4.1: in half of the cases a URI of RFC3986URI, and otherwise a relative
// reference of §4.2: a network-path reference that starts with "//" and an
// authority, an absolute-path reference, a relative-path reference whose
// first segment holds no colon, or an empty path, each with an optional
// query and fragment. Values shrink toward "http://a"; the shrinker keeps
// the form, so a relative reference shrinks toward "//a", "/", "a" or the
// empty reference.
func RFC3986Reference() pbt.Gen[string] {
	uris := RFC3986URI()
	addrs6, addrs4 := pbt.IPv6(), pbt.IPv4()
	relative := pbt.Custom("relative-ref", func(t *pbt.T) string {
		var b strings.Builder
		switch relativeForm.Draw(t, "") {
		case 0:
			b.WriteString("//")
			writeAuthority(t, &b, addrs4, addrs6)
			writeSegments(t, &b)
		case 1:
			writePathAbsolute(t, &b)
		case 2:
			b.WriteString(segmentNZNC.Draw(t, ""))
			writeSegments(t, &b)
		}
		writeQueryFragment(t, &b)
		return b.String()
	})
	return pbt.Custom("RFC3986Reference()", func(t *pbt.T) string {
		if pbt.Bool().Draw(t, "") {
			return relative.Draw(t, "")
		}
		return uris.Draw(t, "")
	})
}

func writeAuthority(t *pbt.T, b *strings.Builder, addrs4, addrs6 pbt.Gen[netip.Addr]) {
	if optional.Draw(t, "") == 1 {
		b.WriteString(userinfo.Draw(t, ""))
		b.WriteByte('@')
	}
	switch hostForm.Draw(t, "") {
	case 0:
		if regNameForm.Draw(t, "") == 0 {
			b.WriteString(hostname(t, alnum, 253))
		} else {
			b.WriteString(regName.Draw(t, ""))
		}
	case 1:
		b.WriteString(addrs4.Draw(t, "").String())
	case 2:
		b.WriteString("[" + ipv6Text(t, addrs6, 1) + "]")
	default:
		fmt.Fprintf(b, "[v%s.%s]", hexText.Draw(t, ""), futureText.Draw(t, ""))
	}
	if optional.Draw(t, "") == 1 {
		b.WriteByte(':')
		b.WriteString(portText.Draw(t, ""))
	}
}

// writeSegments writes path-abempty: any number of "/" segment.
func writeSegments(t *pbt.T, b *strings.Builder) {
	for _, s := range segments.Draw(t, "") {
		b.WriteByte('/')
		b.WriteString(s)
	}
}

// writePathAbsolute writes path-absolute: "/" and an optional non-empty
// segment followed by further segments.
func writePathAbsolute(t *pbt.T, b *strings.Builder) {
	b.WriteByte('/')
	if optional.Draw(t, "") == 1 {
		b.WriteString(segmentNZ.Draw(t, ""))
		writeSegments(t, b)
	}
}

func writeQueryFragment(t *pbt.T, b *strings.Builder) {
	if optional.Draw(t, "") == 1 {
		b.WriteByte('?')
		b.WriteString(queryText.Draw(t, ""))
	}
	if optional.Draw(t, "") == 1 {
		b.WriteByte('#')
		b.WriteString(queryText.Draw(t, ""))
	}
}

var (
	urlScheme   = choice(1, 1) // http, https
	urlHost     = choice(8, 1, 1)
	urlPort     = pbt.IntRange(1, 65535)
	urlSegments = pbt.SliceOfN(pctString(unreserved+"!$&'()*+,;=:@", 0, 12), 0, 4)
	urlParams   = pbt.SliceOfN(pbt.Custom("param", func(t *pbt.T) string {
		return urlKey.Draw(t, "") + "=" + urlValue.Draw(t, "")
	}), 0, 4)
	urlKey   = pctString(unreserved, 1, 8)
	urlValue = pctString(unreserved+"!$'()*+,;:@/?", 0, 12)
)

// URL generates absolute http and https URLs, the subset of RFC 3986 that
// web clients and servers exchange: the scheme in lower case, "://", a host
// that is a host name of Hostname, an IPv4 address, or an IPv6 address in
// the form of RFC 5952 in brackets, an optional port from 1 to 65535, a
// path of segments of unreserved characters, sub-delims and percent-encoded
// octets, an optional query of key=value pairs joined by "&", and an
// optional fragment. url.Parse reads every value with the scheme and host
// it was built from. Values shrink toward "http://a".
func URL() pbt.Gen[string] {
	addrs6, addrs4 := pbt.IPv6(), pbt.IPv4()
	return pbt.Custom("URL()", func(t *pbt.T) string {
		var b strings.Builder
		b.WriteString([...]string{"http://", "https://"}[urlScheme.Draw(t, "")])
		switch urlHost.Draw(t, "") {
		case 0:
			b.WriteString(hostname(t, alnum, 253))
		case 1:
			b.WriteString(addrs4.Draw(t, "").String())
		default:
			b.WriteString("[" + addrs6.Draw(t, "").String() + "]")
		}
		if optional.Draw(t, "") == 1 {
			fmt.Fprintf(&b, ":%d", urlPort.Draw(t, ""))
		}
		for _, s := range urlSegments.Draw(t, "") {
			b.WriteByte('/')
			b.WriteString(s)
		}
		if params := urlParams.Draw(t, ""); len(params) > 0 {
			b.WriteByte('?')
			b.WriteString(strings.Join(params, "&"))
		}
		if optional.Draw(t, "") == 1 {
			b.WriteByte('#')
			b.WriteString(urlValue.Draw(t, ""))
		}
		return b.String()
	})
}
