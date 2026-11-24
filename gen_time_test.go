// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the Zone tests must not depend on the system's database
)

func TestTimeStaysInRangeForArbitraryChoices(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	lo := time.Date(2021, time.March, 14, 1, 59, 59, 500, ny)
	hi := time.Date(2021, time.November, 7, 1, 30, 0, 0, ny)
	cases := []struct {
		name   string
		g      Gen[time.Time]
		lo, hi time.Time
		loc    *time.Location
	}{
		{"Time", Time(), minTime, maxTime, time.UTC},
		{"TimeRange in a DST zone", TimeRange(lo, hi), lo, hi, ny},
		{"TimeRange of one instant", TimeRange(lo, lo), lo, lo, ny},
		{"TimeRange inside a second", TimeRange(lo, lo.Add(100)), lo, lo.Add(100), ny},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			forArbitraryChoices(t, c.g, 3000, func(v time.Time) string {
				switch {
				case v.Before(c.lo) || v.After(c.hi):
					return "outside the range"
				case v.Location() != c.loc:
					return "in location " + v.Location().String()
				case v != v.Round(0):
					return "carries a monotonic clock reading"
				}
				return ""
			})
		})
	}
}

func TestTimeCoversCalendarEdges(t *testing.T) {
	Check(t, func(pt *T) {
		v := Time().Draw(pt, "v")
		next := v.Add(1)
		pt.Cover(v.IsZero(), 1, "zero time")
		pt.Cover(v.Equal(maxTime), 1, "last instant")
		pt.Cover(next.Day() == 1 && next.Hour() == 0 && next.Minute() == 0 && next.Second() == 0 && next.Nanosecond() == 0, 3, "last nanosecond of a month")
		pt.Cover(v.Month() == time.February && v.Day() == 29, 2, "February 29")
		pt.Cover(v.Nanosecond() == 0, 15, "whole second")
		pt.Cover(v.Nanosecond()%1e6 == 0 && v.Nanosecond() != 0, 2, "whole millisecond")
		pt.Cover(v.Year() > 3000, 1, "after 3000")
		pt.Cover(v.Before(time.Unix(0, 0)), 5, "before the epoch")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
}

func TestTimeRangeSnapsToZoneTransitions(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	lo := time.Date(2020, time.January, 1, 0, 0, 0, 0, ny)
	hi := time.Date(2024, time.January, 1, 0, 0, 0, 0, ny)
	Check(t, func(pt *T) {
		v := TimeRange(lo, hi).Draw(pt, "v")
		start, end := v.ZoneBounds()
		pt.Cover(v.Equal(start), 2, "at a transition")
		pt.Cover(v.Add(1).Equal(end), 2, "the nanosecond before a transition")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
}

func TestTimeShrinksTowardTheEpoch(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	if got := minimalOf(t, Time(), func(time.Time) bool { return true }); !got.Equal(epoch) {
		t.Errorf("always failing: got %v, want %v", got, epoch)
	}
	want := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	if got := minimalOf(t, Time(), func(v time.Time) bool { return v.Year() >= 2000 }); !got.Equal(want) {
		t.Errorf("year >= 2000: got %v, want %v", got, want)
	}
	lo := time.Date(2030, time.June, 1, 12, 0, 0, 5, time.UTC)
	if got := minimalOf(t, TimeRange(lo, lo.AddDate(1, 0, 0)), func(time.Time) bool { return true }); !got.Equal(lo) {
		t.Errorf("range after the epoch: got %v, want %v", got, lo)
	}
}

func TestTimeRangePanicsOnReversedBounds(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "lo is after hi") {
			t.Errorf("panic %q", msg)
		}
	}()
	TimeRange(maxTime, minTime)
}

func TestDurationRange(t *testing.T) {
	lo, hi := -90*time.Minute, 36*time.Hour+7
	forArbitraryChoices(t, DurationRange(lo, hi), 3000, func(d time.Duration) string {
		if d < lo || d > hi {
			return "outside the range"
		}
		return ""
	})
	forArbitraryChoices(t, DurationRange(5*time.Second+1, 5*time.Second+3), 300, func(d time.Duration) string {
		if d < 5*time.Second+1 || d > 5*time.Second+3 {
			return "truncated outside the range"
		}
		return ""
	})
	Check(t, func(pt *T) {
		d := Duration().Draw(pt, "d")
		pt.Cover(d%time.Second == 0 && d != 0, 10, "whole seconds")
		pt.Cover(d == -1<<63 || d == 1<<63-1, 1, "extreme")
		pt.Cover(d < 0, 20, "negative")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
	if got := minimalOf(t, Duration(), func(d time.Duration) bool { return d >= time.Hour }); got != time.Hour {
		t.Errorf("d >= 1h: got %v", got)
	}
	if got := minimalOf(t, DurationRange(time.Second, time.Minute), func(time.Duration) bool { return true }); got != time.Second {
		t.Errorf("range above 0: got %v", got)
	}
}

func TestFixedZone(t *testing.T) {
	forArbitraryChoices(t, FixedZone(), 2000, func(loc *time.Location) string {
		_, off := time.Date(2000, 1, 1, 0, 0, 0, 0, loc).Zone()
		switch {
		case off < -12*3600 || off > 14*3600:
			return "offset out of range"
		case off%(15*60) != 0:
			return "offset not a quarter hour"
		case off == 0 && loc != time.UTC:
			return "zero offset is not time.UTC"
		}
		return ""
	})
	if got := minimalOf(t, FixedZone(), func(*time.Location) bool { return true }); got != time.UTC {
		t.Errorf("shrinks to %v", got)
	}
	if got := fixedZones[48+4*9].String(); got != "+09:00" {
		t.Errorf("name of +9h: %q", got)
	}
	if got := fixedZones[48-14].String(); got != "-03:30" {
		t.Errorf("name of -3.5h: %q", got)
	}
}

func TestZone(t *testing.T) {
	g := Zone()
	if g.String() != "Zone()" {
		t.Errorf("name %q", g)
	}
	if got := minimalOf(t, g, func(*time.Location) bool { return true }); got != time.UTC {
		t.Errorf("default list shrinks to %v", got)
	}
	tokyo := Zone("Asia/Tokyo", "Europe/London")
	if tokyo.String() != "Zone(Asia/Tokyo, Europe/London)" {
		t.Errorf("name %q", tokyo)
	}
	if got := minimalOf(t, tokyo, func(*time.Location) bool { return true }); got.String() != "Asia/Tokyo" {
		t.Errorf("named list shrinks to %v", got)
	}
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "time/tzdata") {
			t.Errorf("panic %q", msg)
		}
	}()
	Zone("Nowhere/Atlantis")
}
