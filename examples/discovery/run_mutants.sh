#!/bin/sh
# SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
# SPDX-License-Identifier: Apache-2.0
#
# 通常のテストを実行してから、TestXxxMutant を1本ずつ実行し、失敗することを確かめる。
# 失敗するはずの変異が通ったら、そのテスト名を表示して終了コード1を返す。
# 使い方: examples ディレクトリで ./discovery/run_mutants.sh [パッケージのパターン]
pkgs="${1:-./discovery/...}"
go vet $pkgs || exit 1
go test -count=1 $pkgs || exit 1
fail=0
total=0
for pkg in $(go list $pkgs); do
    for name in $(go test -list 'Mutant$' "$pkg" | grep '^Test'); do
        total=$((total + 1))
        if PBT_MUTANTS=1 go test -count=1 -run "^${name}\$" "$pkg" >/dev/null 2>&1; then
            echo "PASSED UNEXPECTEDLY: $pkg $name"
            fail=1
        fi
    done
done
echo "mutants run: $total"
exit $fail
