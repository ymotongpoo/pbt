// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"net"
	"net/netip"
	"strings"
	"testing"
)

func TestAddressesAreValidForArbitraryChoices(t *testing.T) {
	forArbitraryChoices(t, IPv4(), 3000, func(a netip.Addr) string {
		if !a.IsValid() || !a.Is4() {
			return "not an IPv4 address"
		}
		return ""
	})
	forArbitraryChoices(t, IPv6(), 3000, func(a netip.Addr) string {
		if !a.IsValid() || !a.Is6() || a.Zone() != "" {
			return "not a zoneless IPv6 address"
		}
		return ""
	})
	forArbitraryChoices(t, Prefix(), 3000, func(p netip.Prefix) string {
		if !p.IsValid() {
			return "invalid prefix"
		}
		return ""
	})
	forArbitraryChoices(t, AddrPort(), 3000, func(ap netip.AddrPort) string {
		if !ap.IsValid() {
			return "invalid address and port"
		}
		return ""
	})
}

func TestAddrInStaysInItsPrefixes(t *testing.T) {
	ps := []netip.Prefix{
		netip.MustParsePrefix("10.1.2.3/8"),
		netip.MustParsePrefix("192.168.7.0/31"),
		netip.MustParsePrefix("203.0.113.9/32"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2001:db8:aa::/56"),
		netip.MustParsePrefix("::/0"),
		netip.MustParsePrefix("0.0.0.0/0"),
	}
	for _, p := range ps {
		t.Run(p.String(), func(t *testing.T) {
			forArbitraryChoices(t, AddrIn(p), 2000, func(a netip.Addr) string {
				if !p.Masked().Contains(a) || a.Is4() != p.Addr().Is4() {
					return "outside " + p.String()
				}
				return ""
			})
		})
	}
	if got := minimalOf(t, AddrIn(ps[3], ps[0]), func(netip.Addr) bool { return true }); got != netip.MustParseAddr("2001:db8::") {
		t.Errorf("shrinks to %v", got)
	}
}

func TestAddrInPanics(t *testing.T) {
	for _, c := range []struct {
		name string
		f    func()
		want string
	}{
		{"none", func() { AddrIn() }, "at least one prefix"},
		{"invalid", func() { AddrIn(netip.Prefix{}) }, "invalid prefix"},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if msg, _ := recover().(string); !strings.Contains(msg, c.want) {
					t.Errorf("panic %q", msg)
				}
			}()
			c.f()
		})
	}
}

func TestAddressesCoverSpecialBlocks(t *testing.T) {
	Check(t, func(pt *T) {
		a := IPv4().Draw(pt, "a")
		pt.Cover(a.IsPrivate(), 3, "private")
		pt.Cover(a.IsLoopback(), 1, "loopback")
		pt.Cover(a.IsMulticast(), 1, "multicast")
		pt.Cover(a.IsLinkLocalUnicast(), 1, "link local")
		pt.Cover(a.As4()[3] == 0, 5, "last octet 0")
		pt.Cover(a.As4()[3] == 255, 3, "last octet 255")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
	Check(t, func(pt *T) {
		a := IPv6().Draw(pt, "a")
		pt.Cover(a.Is4In6(), 1, "IPv4-mapped")
		pt.Cover(a.IsLoopback(), 1, "loopback")
		pt.Cover(a.IsLinkLocalUnicast(), 1, "link local")
		pt.Cover(a.IsPrivate(), 1, "unique local")
		pt.Cover(a.IsMulticast(), 2, "multicast")
		pt.Cover(strings.Contains(a.String(), "::"), 20, "compressible")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
	Check(t, func(pt *T) {
		p := Prefix().Draw(pt, "p")
		pt.Cover(p.Bits() == 0, 3, "/0")
		pt.Cover(p.Bits() == p.Addr().BitLen(), 3, "single address")
		pt.Cover(p != p.Masked(), 30, "host bits set")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
}

func TestAddressesShrink(t *testing.T) {
	cases := []struct {
		name string
		g    Gen[netip.Addr]
		want string
	}{
		{"IPv4", IPv4(), "0.0.0.0"},
		{"IPv6", IPv6(), "::"},
		{"Addr", Addr(), "0.0.0.0"},
	}
	for _, c := range cases {
		if got := minimalOf(t, c.g, func(netip.Addr) bool { return true }); got.String() != c.want {
			t.Errorf("%s shrinks to %v, want %s", c.name, got, c.want)
		}
	}
	if got := minimalOf(t, IPv4(), func(a netip.Addr) bool { return a.IsPrivate() }); got.String() != "10.0.0.0" {
		t.Errorf("private IPv4 shrinks to %v", got)
	}
	if got := minimalOf(t, Prefix(), func(netip.Prefix) bool { return true }); got.String() != "0.0.0.0/0" {
		t.Errorf("Prefix shrinks to %v", got)
	}
	if got := minimalOf(t, AddrPort(), func(ap netip.AddrPort) bool { return ap.Port() > 1023 }); got.String() != "0.0.0.0:1024" {
		t.Errorf("AddrPort shrinks to %v", got)
	}
}

func TestHardwareAddr(t *testing.T) {
	forArbitraryChoices(t, HardwareAddr(), 3000, func(mac net.HardwareAddr) string {
		if len(mac) != 6 && len(mac) != 8 {
			return "length is not 6 or 8"
		}
		back, err := net.ParseMAC(mac.String())
		if err != nil || back.String() != mac.String() {
			return "does not round-trip through net.ParseMAC"
		}
		return ""
	})
	Check(t, func(pt *T) {
		mac := HardwareAddr().Draw(pt, "mac")
		pt.Cover(len(mac) == 8, 3, "EUI-64")
		pt.Cover(mac.String() == "ff:ff:ff:ff:ff:ff", 2, "broadcast")
		pt.Cover(mac[0]&1 == 1, 10, "group bit")
		pt.Cover(mac[0]&2 == 2, 10, "local bit")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
	if got := minimalOf(t, HardwareAddr(), func(net.HardwareAddr) bool { return true }); got.String() != "00:00:00:00:00:00" {
		t.Errorf("shrinks to %v", got)
	}
	if got := minimalOf(t, HardwareAddr(), func(m net.HardwareAddr) bool { return m[0]&1 == 1 }); got.String() != "01:00:00:00:00:00" {
		t.Errorf("multicast shrinks to %v", got)
	}
	// Every draw owns its slice.
	g := HardwareAddr()
	a, b := g.Example(1), g.Example(1)
	a[0] ^= 0xff
	if a[0] == b[0] {
		t.Error("two draws share a slice")
	}
}
