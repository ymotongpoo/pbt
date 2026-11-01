// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

//go:build !race

package ch16

import (
	"testing"

	"go.ymotongpoo.com/pbt"
)

// -race なしでは、戻り値が説明できるので通る。
func TestStatCounterSurvives(t *testing.T) {
	pbt.Check(t, counterProperty(func() adder { return &StatCounter{} }), pbt.NoSave(), pbt.NoReplay())
}
