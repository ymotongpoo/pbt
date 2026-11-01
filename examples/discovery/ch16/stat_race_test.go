// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

//go:build race

package ch16

import (
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// -race を付けると、データ競合の報告で失敗する。
func TestStatCounterMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, counterProperty(func() adder { return &StatCounter{} }), pbt.NoSave(), pbt.NoReplay())
}
