// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"flag"
	"time"
)

var flagBudget = flag.Duration("pbt.budget", 0, "time budget for searching for cases (0 means unlimited)")

// Budget limits the elapsed search time of Check and CheckExists. Zero, the
// default, disables the limit; negative durations are invalid. The flag
// -pbt.budget accepts the same duration. The clock starts after saved-case
// replay, and the budget is checked before each new case, including discards,
// extended CheckCoverage runs and targeted SearchSteps. A running case is
// allowed to finish, so Budget cannot interrupt a blocked property.
//
// A Check passes with the cases completed before the budget expires, unless
// it has found failures or an enabled coverage or health check fails. An
// undecided Cover requirement fails under CheckCoverage. CheckExists fails
// if no witness was found. With MaxFailures, all collected failures are still
// shrunk and reported. Replay and shrinking are outside this budget;
// ShrinkTime separately bounds each failure's or witness's shrinking.
//
// The case limit still applies. To search for approximately d, use Budget(d)
// with a large Checks value (and a large SearchSteps for targeted properties).
// Budget does not promise a minimum runtime. A run cut short always logs that
// the budget ended the search, even without Verbose or statistics.
func Budget(d time.Duration) Option { return func(c *config) { c.budget = d } }
