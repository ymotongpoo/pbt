// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestIPv4AndRFC5952IPv6AreCanonical(t *testing.T) {
	checkValid(t, IPv4(), func(s string) string {
		if a, err := netip.ParseAddr(s); err != nil || !a.Is4() || a.String() != s {
			return "not canonical dotted decimal"
		}
		return ""
	})
	checkValid(t, RFC5952IPv6(), func(s string) string {
		if a, err := netip.ParseAddr(s); err != nil || !a.Is6() || a.String() != s {
			return "not the RFC 5952 form"
		}
		return ""
	})
	shrinksTo(t, RFC5952IPv6(), always, "::")
}

var (
	ipv6GroupRe = regexp.MustCompile(`^[0-9A-Fa-f]{1,4}$`)
	snumRe      = regexp.MustCompile(`^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$`)
)

// checkSnum checks four decimal octets of one to three digits, with
// leading zeros when leadingZeros is set, and returns them without the
// leading zeros.
func checkSnum(s string, leadingZeros bool) (string, bool) {
	m := snumRe.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	var parts []string
	for _, o := range m[1:] {
		v, _ := strconv.Atoi(o)
		if v > 255 || (!leadingZeros && len(o) > 1 && o[0] == '0') {
			return "", false
		}
		parts = append(parts, strconv.Itoa(v))
	}
	return strings.Join(parts, "."), true
}

// checkRFC4291 checks the text forms of RFC 4291 §2.2 without relying on
// netip, in which "::" stands for at least minRun groups and, when snum is
// set, an embedded IPv4 address may have leading zeros as in RFC 5321;
// then that netip reads an address from it.
func checkRFC4291(s string, minRun int, snum bool) string {
	head, tail, compressed := strings.Cut(s, "::")
	if strings.Contains(tail, "::") {
		return "more than one ::"
	}
	var groups []string
	split := func(part string) {
		if part != "" {
			groups = append(groups, strings.Split(part, ":")...)
		}
	}
	split(head)
	split(tail)
	n := 0
	normal := s
	for i, g := range groups {
		if i == len(groups)-1 && strings.Contains(g, ".") {
			plain, ok := checkSnum(g, snum)
			if !ok {
				return "bad embedded IPv4 " + g
			}
			normal = strings.TrimSuffix(s, g) + plain
			n += 2
			continue
		}
		if !ipv6GroupRe.MatchString(g) {
			return "bad group " + g
		}
		n++
	}
	switch {
	case !compressed && n != 8:
		return "not eight groups"
	case compressed && 8-n < minRun:
		return ":: stands for too few groups"
	}
	if a, err := netip.ParseAddr(normal); err != nil || !a.Is6() {
		return "netip.ParseAddr rejects it"
	}
	return ""
}

func TestRFC4291IPv6(t *testing.T) {
	checkValid(t, RFC4291IPv6(), func(s string) string { return checkRFC4291(s, 1, false) })
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC4291IPv6().Draw(pt, "s")
		a, _ := netip.ParseAddr(s)
		pt.Cover(a.String() != s, 50, "not canonical")
		pt.Cover(strings.ContainsAny(s, "ABCDEF"), 5, "upper case")
		pt.Cover(strings.Contains(s, "::"), 30, "compressed")
		pt.Cover(!strings.Contains(s, "::"), 10, "not compressed")
		pt.Cover(strings.Contains(s, "."), 10, "embedded IPv4")
		pt.Cover(regexp.MustCompile(`(^|:)0[0-9a-fA-F]`).MatchString(s), 10, "leading zero")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC4291IPv6(), always, "::")
}

func TestCIDR(t *testing.T) {
	checkValid(t, CIDR(), func(s string) string {
		if p, err := netip.ParsePrefix(s); err != nil || p != p.Masked() || p.String() != s {
			return "not a canonical masked prefix"
		}
		return ""
	})
	shrinksTo(t, CIDR(), always, "0.0.0.0/0")
}

func TestMAC(t *testing.T) {
	checkValid(t, MAC(), func(s string) string {
		if _, err := net.ParseMAC(s); err != nil {
			return err.Error()
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := MAC().Draw(pt, "s")
		pt.Cover(strings.Contains(s, "-"), 10, "hyphens")
		pt.Cover(strings.Contains(s, "."), 5, "dots")
		pt.Cover(strings.ContainsAny(s, "ABCDEF"), 5, "upper case")
	}, pbt.Seed(1), pbt.Checks(1000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, MAC(), always, "00:00:00:00:00:00")
}

var (
	hostnameRe = regexp.MustCompile(`^(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)*[A-Za-z](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	domainRe   = regexp.MustCompile(`^(?:[A-Za-z](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)*[A-Za-z](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
)

func TestHostname(t *testing.T) {
	checkValid(t, Hostname(), func(s string) string {
		if !hostnameRe.MatchString(s) || len(s) > 253 {
			return "not an RFC 1123 host name"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := Hostname().Draw(pt, "s")
		labels := strings.Split(s, ".")
		pt.Cover(len(s) == 253, 2, "253 characters")
		pt.Cover(len(labels) > 1, 30, "several labels")
		pt.Cover(strings.ContainsAny(s[:1], digits) && len(labels) > 1, 2, "label starting with a digit")
		pt.Cover(strings.Contains(s, "-"), 10, "hyphen")
		long := false
		for _, l := range labels {
			long = long || len(l) == 63
		}
		pt.Cover(long, 3, "label of 63 characters")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, Hostname(), always, "a")
	checkValid(t, RFC1035Domain(), func(s string) string {
		if !domainRe.MatchString(s) || len(s) > 253 {
			return "not an RFC 1035 domain name"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		pt.Cover(strings.Count(RFC1035Domain().Draw(pt, "s"), ".") >= 5, 3, "six labels or more")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC1035Domain(), always, "a")
}
