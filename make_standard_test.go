// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMakeStandardTypes(t *testing.T) {
	type standard struct {
		Time    time.Time
		Zone    *time.Location
		Addr    netip.Addr
		Prefix  netip.Prefix
		Port    netip.AddrPort
		IP      net.IP
		MAC     net.HardwareAddr
		Bounded time.Time `pbt:"min=2020-01-01T01:00:00+01:00,max=2021-01-01T00:00:00Z"`
	}
	seen := map[string]bool{}
	Check(t, func(pt *T) {
		v := Make[standard]().Draw(pt, "v")
		if v.Zone == nil || !v.Addr.IsValid() || !v.Prefix.IsValid() || !v.Port.IsValid() || v.IP.To16() == nil || (len(v.MAC) != 6 && len(v.MAC) != 8) {
			pt.Fatalf("invalid standard types: %+v", v)
		}
		if v.Bounded.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) || v.Bounded.After(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)) {
			pt.Fatalf("time outside bounds: %s", v.Bounded)
		}
		seen[fmt.Sprintf("ip%d-v4%v", len(v.IP), v.IP.To4() != nil)] = true
		seen[fmt.Sprintf("time-nonzero%v", !v.Time.IsZero())] = true
	}, Seed(1), Checks(500), NoSave())
	for _, key := range []string{"ip4-v4true", "ip16-v4true", "ip16-v4false", "time-nonzerotrue"} {
		if !seen[key] {
			t.Errorf("never generated %s", key)
		}
	}
}

func TestMakeStandardTypeOverrides(t *testing.T) {
	type event struct {
		At    time.Time
		Other time.Time
		Zone  *time.Location
	}
	when := time.Unix(123, 456).UTC()
	field := when.Add(time.Hour)
	g := Make[event](WithType(Just(when)), WithField[event]("At", Just(field)), WithType(Just(time.UTC)))
	v := g.Example(1)
	if !v.At.Equal(field) || !v.Other.Equal(when) || v.Zone != time.UTC {
		t.Fatalf("override precedence: %+v", v)
	}
	if got := Make[time.Time](WithType(Just(when))).Example(1); got != when {
		t.Fatalf("top-level override: %s", got)
	}
}

func TestMakeTimeTagErrors(t *testing.T) {
	for _, tag := range []string{"min=bad", "max=bad", "min=2021-01-01T00:00:00Z,max=2020-01-01T00:00:00Z"} {
		t.Run(tag, func(t *testing.T) {
			defer func() {
				if p := recover(); p == nil || !strings.Contains(fmt.Sprint(p), "pbt: Make:") {
					t.Errorf("wrong panic: %v", p)
				}
			}()
			makeDraw(reflect.TypeFor[time.Time](), &makeConfig{}, parseTag(tag), nil)
		})
	}
	for _, tag := range []string{"min=2020-01-01T00:00:00Z", "max=2020-01-01T00:00:00Z"} {
		if standardDraw(reflect.TypeFor[time.Time](), parseTag(tag)) == nil {
			t.Fatal("one-sided time bound unsupported")
		}
	}
}

func TestMakeDurationKeepsIntegerChoices(t *testing.T) {
	for _, seed := range []uint64{1, 3, 5} {
		a := Make[time.Duration]().Example(seed)
		b := Make[int64]().Example(seed)
		if int64(a) != b {
			t.Fatalf("duration generation changed: %v != %v", a, b)
		}
	}
}

func TestMakeTimeTagUsesFixedOffset(t *testing.T) {
	old := time.Local
	time.Local = time.FixedZone("host zone", 3600)
	defer func() { time.Local = old }()
	v := Make[struct {
		At time.Time `pbt:"min=2020-01-01T01:00:00+01:00,max=2020-01-01T01:00:00+01:00"`
	}]().Example(1)
	if v.At.Location() == time.Local {
		t.Fatal("numeric bound inherited the host's local location")
	}
	name, offset := v.At.Zone()
	if name != "" || offset != 3600 {
		t.Fatalf("zone=(%q,%d)", name, offset)
	}
}
