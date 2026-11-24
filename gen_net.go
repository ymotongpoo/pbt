// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// specialIPv4 lists the IPv4 blocks of the IANA special-purpose address
// registry (RFC 6890 and its updates) together with multicast (RFC 5771)
// and the reserved class E block (RFC 1112). Addresses in them follow
// special rules, which is why IPv4 draws from them in a share of the cases.
var specialIPv4 = mustPrefixes(
	"0.0.0.0/8",          // "this network" (RFC 791)
	"10.0.0.0/8",         // private use (RFC 1918)
	"100.64.0.0/10",      // shared address space for carrier-grade NAT (RFC 6598)
	"127.0.0.0/8",        // loopback (RFC 1122)
	"169.254.0.0/16",     // link local (RFC 3927)
	"172.16.0.0/12",      // private use (RFC 1918)
	"192.0.0.0/24",       // IETF protocol assignments (RFC 6890)
	"192.0.2.0/24",       // documentation, TEST-NET-1 (RFC 5737)
	"192.88.99.0/24",     // 6to4 relay anycast, deprecated (RFC 7526)
	"192.168.0.0/16",     // private use (RFC 1918)
	"198.18.0.0/15",      // benchmarking (RFC 2544)
	"198.51.100.0/24",    // documentation, TEST-NET-2 (RFC 5737)
	"203.0.113.0/24",     // documentation, TEST-NET-3 (RFC 5737)
	"224.0.0.0/4",        // multicast (RFC 5771)
	"240.0.0.0/4",        // reserved (RFC 1112)
	"255.255.255.255/32", // limited broadcast (RFC 919)
)

// specialIPv6 lists the IPv6 blocks of the IANA special-purpose address
// registry (RFC 6890 and its updates) together with multicast (RFC 4291).
var specialIPv6 = mustPrefixes(
	"::/128",             // unspecified (RFC 4291)
	"::1/128",            // loopback (RFC 4291)
	"::ffff:0:0/96",      // IPv4-mapped (RFC 4291)
	"64:ff9b::/96",       // IPv4/IPv6 translation (RFC 6052)
	"64:ff9b:1::/48",     // local-use IPv4/IPv6 translation (RFC 8215)
	"100::/64",           // discard only (RFC 6666)
	"2001::/32",          // Teredo (RFC 4380)
	"2001:db8::/32",      // documentation (RFC 3849)
	"2002::/16",          // 6to4 (RFC 3056)
	"fc00::/7",           // unique local (RFC 4193)
	"fe80::/10",          // link-local unicast (RFC 4291)
	"ff00::/8",           // multicast (RFC 4291)
	"ff02::1/128",        // all-nodes multicast (RFC 4291)
	"ff02::1:ff00:0/104", // solicited-node multicast (RFC 4291)
)

func mustPrefixes(ss ...string) []netip.Prefix {
	ps := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		ps[i] = netip.MustParsePrefix(s)
	}
	return ps
}

// IPv4 generates IPv4 addresses. Each octet is drawn like a Byte, so 0 and
// 255 are common, and about one address in five comes from a special-purpose
// block: "this network", private use, shared address space, loopback, link
// local, the documentation networks, benchmarking, multicast, the reserved
// class E block and the limited broadcast address. Values shrink toward
// 0.0.0.0.
func IPv4() Gen[netip.Addr] {
	special := AddrIn(specialIPv4...)
	return Gen[netip.Addr]{name: "IPv4()", draw: func(t *T) netip.Addr {
		if t.drawIndex([]int{4, 1}, 5) == 1 {
			return special.draw(t)
		}
		var b [4]byte
		for i := range b {
			b[i] = byte(t.draw(255, plainMeta, modeBiased))
		}
		return netip.AddrFrom4(b)
	}}
}

// IPv6 generates IPv6 addresses without a zone. Each of the eight 16-bit
// groups is drawn like a Uint16, so zero groups, which the text form
// compresses with "::", are common, and about one address in three comes
// from a special-purpose block: the unspecified and loopback addresses,
// IPv4-mapped addresses (for which Is4In6 reports true), the translation
// prefixes, discard only, Teredo, documentation, 6to4, unique local,
// link-local and multicast. Values shrink toward ::.
func IPv6() Gen[netip.Addr] {
	special := AddrIn(specialIPv6...)
	return Gen[netip.Addr]{name: "IPv6()", draw: func(t *T) netip.Addr {
		if t.drawIndex([]int{2, 1}, 3) == 1 {
			return special.draw(t)
		}
		var b [16]byte
		for i := 0; i < 16; i += 2 {
			g := t.draw(0xffff, plainMeta, modeBiased)
			b[i], b[i+1] = byte(g>>8), byte(g)
		}
		return netip.AddrFrom16(b)
	}}
}

// Addr generates IPv4 and IPv6 addresses in equal shares, drawn by IPv4 and
// IPv6. Values shrink toward 0.0.0.0.
func Addr() Gen[netip.Addr] {
	return named(OneOf(IPv4(), IPv6()), "Addr()")
}

// AddrIn generates addresses inside the given prefixes. A prefix is picked
// with equal probability, then its host bits are drawn like an unsigned
// integer, so the first and the last address of the prefix, such as the
// network and the broadcast address of an IPv4 subnet, are common. The
// host bits of the prefixes are ignored, so 10.1.2.3/8 means 10.0.0.0/8.
// Values shrink toward the first address of the first prefix. AddrIn panics
// without prefixes or with an invalid one.
func AddrIn(prefixes ...netip.Prefix) Gen[netip.Addr] {
	if len(prefixes) == 0 {
		panic("pbt: AddrIn needs at least one prefix")
	}
	ps := make([]netip.Prefix, len(prefixes))
	names := make([]string, len(prefixes))
	for i, p := range prefixes {
		if !p.IsValid() {
			panic(fmt.Sprintf("pbt: AddrIn: invalid prefix %v", p))
		}
		ps[i] = p.Masked()
		names[i] = ps[i].String()
	}
	return Gen[netip.Addr]{name: "AddrIn(" + strings.Join(names, ", ") + ")", draw: func(t *T) netip.Addr {
		p := ps[t.draw(uint64(len(ps)-1), plainMeta, modeUniform)]
		return hostIn(t, p)
	}}
}

// hostIn draws the host bits of p: one choice for up to 64 bits, two for
// more, the high half first.
func hostIn(t *T, p netip.Prefix) netip.Addr {
	host := p.Addr().BitLen() - p.Bits()
	b := p.Addr().As16()
	var hi, lo uint64
	switch {
	case host == 0:
	case host <= 64:
		lo = t.draw(maxBits(host), plainMeta, modeBiased)
	default:
		hi = t.draw(maxBits(host-64), plainMeta, modeBiased)
		lo = t.draw(maxBits(64), plainMeta, modeBiased)
	}
	for i := range 8 {
		b[15-i] |= byte(lo >> (8 * i))
		b[7-i] |= byte(hi >> (8 * i))
	}
	if p.Addr().Is4() {
		return netip.AddrFrom4([4]byte(b[12:]))
	}
	return netip.AddrFrom16(b)
}

func maxBits(n int) uint64 {
	if n >= 64 {
		return 1<<64 - 1
	}
	return 1<<n - 1
}

// Prefix generates IP prefixes: an address from Addr and a prefix length
// from 0 to the address's bit length, with the bounds common. The host bits
// are left as drawn, as in 192.168.1.10/24, which writes an interface
// address together with its subnet; apply netip.Prefix.Masked through Map
// for the canonical network form. Values shrink toward 0.0.0.0/0.
func Prefix() Gen[netip.Prefix] {
	addr := Addr()
	return Gen[netip.Prefix]{name: "Prefix()", draw: func(t *T) netip.Prefix {
		a := addr.draw(t)
		return netip.PrefixFrom(a, int(t.draw(uint64(a.BitLen()), plainMeta, modeBiased)))
	}}
}

// AddrPort generates an address from Addr with a port drawn like a Uint16,
// so port 0 and port 65535 are common. Values shrink toward 0.0.0.0:0.
func AddrPort() Gen[netip.AddrPort] {
	addr := Addr()
	return Gen[netip.AddrPort]{name: "AddrPort()", draw: func(t *T) netip.AddrPort {
		// The port comes first: the address draws a variable number of
		// choices, and a port after it would change whenever shrinking
		// switches the address from IPv6 to IPv4.
		port := uint16(t.draw(0xffff, plainMeta, modeBiased))
		return netip.AddrPortFrom(addr.draw(t), port)
	}}
}

// specialMACs are prefixes of hardware addresses with special meaning: the
// broadcast address, the multicast prefixes of IPv4 (RFC 1112) and IPv6
// (RFC 2464), and the all-zero address.
var specialMACs = [][]byte{
	{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	{0x01, 0x00, 0x5e},
	{0x33, 0x33},
	{0, 0, 0, 0, 0, 0},
}

// HardwareAddr generates IEEE 802 MAC addresses: EUI-48, six octets, in
// most cases and EUI-64, eight octets, in about one case in ten, the two
// lengths net.ParseMAC reads besides the twenty octets of IP over
// InfiniBand. Each octet is drawn like a Byte, so the group and the local
// bits of the first octet vary, and about one address in six is the
// broadcast address, the all-zero address or an IPv4 or IPv6 multicast
// address. Every draw returns a new slice. Values shrink toward
// 00:00:00:00:00:00.
func HardwareAddr() Gen[net.HardwareAddr] {
	return Gen[net.HardwareAddr]{name: "HardwareAddr()", draw: func(t *T) net.HardwareAddr {
		n := 6
		if t.drawIndex([]int{9, 1}, 10) == 1 {
			n = 8
		}
		// The special prefix is drawn before the octets, so that the
		// shrinker does not prefer it to the octets it overwrites.
		k := t.drawIndex([]int{25, 2, 1, 1, 1}, 30)
		mac := make(net.HardwareAddr, n)
		for i := range mac {
			mac[i] = byte(t.draw(255, plainMeta, modeBiased))
		}
		if k > 0 && n == 6 {
			copy(mac, specialMACs[k-1])
		}
		return mac
	}}
}
