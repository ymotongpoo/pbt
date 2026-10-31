// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	typeList := flag.String("type", "", "comma-separated list of type names to generate generators for")
	output := flag.String("output", "", "output file; the default is pbt_gen.go in the package directory")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: pbtgen -type T1,T2 [-output file] [directory]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *typeList == "" {
		flag.Usage()
		os.Exit(2)
	}
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	var names []string
	for _, n := range strings.Split(*typeList, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	out := outputPath(dir, *output)
	src, err := Generate(dir, names, filepath.Base(out))
	if err != nil {
		fmt.Fprintln(os.Stderr, "pbtgen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "pbtgen:", err)
		os.Exit(1)
	}
}
