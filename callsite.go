// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var drawPCs sync.Map // return PC -> first external return PC (or zero for ForAll)

// drawCaller records a return PC, resolving frames only on the first visit.
func drawCaller() uintptr {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // Callers, drawCaller, Draw
	if v, ok := drawPCs.Load(pcs[0]); ok {
		return v.(uintptr)
	}
	var stack [32]uintptr
	n := runtime.Callers(3, stack[:])
	frames := runtime.CallersFrames(stack[:n])
	var pc uintptr
	for {
		f, more := frames.Next()
		// The wrapper, not the caller of Check, owns a ForAll draw.
		if strings.HasPrefix(f.Function, "go.ymotongpoo.com/pbt.ForAll") {
			break
		}
		if f.Function != "" && !internalFrame(f.Function, f.File) {
			pc = f.PC + 1
			break
		}
		if !more {
			break
		}
	}
	drawPCs.Store(pcs[0], pc)
	return pc
}

func drawSite(pc uintptr) (file string, line int) {
	if pc == 0 {
		return "", 0
	}
	f, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	return f.File, f.Line
}

func shortSite(pc uintptr) string {
	file, line := drawSite(pc)
	if file == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

var sourceNames sync.Map // file -> func() map[line]name; errors are cached too

func sourceName(file string, line int) string {
	load, _ := sourceNames.LoadOrStore(file, sync.OnceValue(func() map[int]string {
		return parseDrawNames(file)
	}))
	return load.(func() map[int]string)()[line]
}

// Ambiguous lines (multiple Draw calls) deliberately have no inferred name.
func parseDrawNames(file string) map[int]string {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, file, nil, 0)
	if err != nil {
		return nil
	}
	names, calls := map[int]string{}, map[int]int{}
	isDraw := func(e ast.Expr) (*ast.CallExpr, bool) {
		c, ok := e.(*ast.CallExpr)
		if !ok {
			return nil, false
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		return c, ok && sel.Sel.Name == "Draw"
	}
	add := func(id *ast.Ident, e ast.Expr) {
		c, ok := isDraw(e)
		if !ok || id == nil || id.Name == "_" {
			return
		}
		for line := fs.Position(c.Pos()).Line; line <= fs.Position(c.End()).Line; line++ {
			names[line] = id.Name
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if c, ok := isDraw(n); ok {
				for line := fs.Position(c.Pos()).Line; line <= fs.Position(c.End()).Line; line++ {
					calls[line]++
				}
			}
		case *ast.AssignStmt:
			if len(n.Lhs) == 1 && len(n.Rhs) == 1 {
				id, _ := n.Lhs[0].(*ast.Ident)
				add(id, n.Rhs[0])
			}
		case *ast.ValueSpec:
			if len(n.Names) == 1 && len(n.Values) == 1 {
				add(n.Names[0], n.Values[0])
			}
		}
		return true
	})
	for line := range names {
		if calls[line] != 1 {
			delete(names, line)
		}
	}
	return names
}

func drawName(d drawRecord) string {
	if d.label != "" {
		return d.label
	}
	file, line := drawSite(d.pc)
	if file == "" {
		return ""
	}
	return sourceName(file, line)
}
