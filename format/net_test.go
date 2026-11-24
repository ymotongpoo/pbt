// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net"
	"net/netip"
	"regexp"
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
	if got := minimalOf(t, RFC5952IPv6(), always); got != "::" {
		t.Errorf("RFC5952IPv6 shrinks to %q", got)
	}
}

var ipv6GroupRe = regexp.MustCompile(`^[0-9A-Fa-f]{1,4}$`)

// checkRFC4291 checks the text forms of RFC 4291 §2.2 without relying on
// netip, then that netip reads the same address.
func checkRFC4291(s string, minRun int) string {
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
	for i, g := range groups {
		if i == len(groups)-1 && strings.Contains(g, ".") {
			if a, err := netip.ParseAddr(g); err != nil || !a.Is4() {
				return "bad embedded IPv4 " + g
			}
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
	if a, err := netip.ParseAddr(s); err != nil || !a.Is6() {
		return "netip.ParseAddr rejects it"
	}
	return ""
}

func TestRFC4291IPv6(t *testing.T) {
	checkValid(t, RFC4291IPv6(), func(s string) string { return checkRFC4291(s, 1) })
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
	if got := minimalOf(t, RFC4291IPv6(), always); got != "::" {
		t.Errorf("shrinks to %q", got)
	}
}

func TestCIDR(t *testing.T) {
	checkValid(t, CIDR(), func(s string) string {
		if p, err := netip.ParsePrefix(s); err != nil || p != p.Masked() || p.String() != s {
			return "not a canonical masked prefix"
		}
		return ""
	})
	if got := minimalOf(t, CIDR(), always); got != "0.0.0.0/0" {
		t.Errorf("shrinks to %q", got)
	}
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
	if got := minimalOf(t, MAC(), always); got != "00:00:00:00:00:00" {
		t.Errorf("shrinks to %q", got)
	}
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
	if got := minimalOf(t, Hostname(), always); got != "a" {
		t.Errorf("shrinks to %q", got)
	}
	checkValid(t, RFC1035Domain(), func(s string) string {
		if !domainRe.MatchString(s) || len(s) > 253 {
			return "not an RFC 1035 domain name"
		}
		return ""
	})
	if got := minimalOf(t, RFC1035Domain(), always); got != "a" {
		t.Errorf("RFC1035Domain shrinks to %q", got)
	}
}
