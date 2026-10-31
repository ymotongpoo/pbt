// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch10bookstore

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// model maps each ISBN to the row the store should hold. It is immutable:
// every update returns a copy.
type model map[string]Book

func (m model) with(b Book) model {
	out := maps.Clone(m)
	if out == nil {
		out = model{}
	}
	out[b.ISBN] = b
	return out
}

func (m model) isbns() []string { return slices.Sorted(maps.Keys(m)) }

// The ISBN generators of the chapter: a group from the registered ranges,
// the remaining digits split between publisher and title, and a check
// character, with an optional 978 or 979 prefix.
var (
	group = pbt.OneOf(
		pbt.Map(pbt.IntRange(0, 5), itoa), pbt.Just("7"), pbt.Map(pbt.IntRange(80, 94), itoa),
		pbt.Map(pbt.IntRange(600, 621), itoa), pbt.Map(pbt.IntRange(950, 989), itoa),
		pbt.Map(pbt.IntRange(9926, 9989), itoa), pbt.Map(pbt.IntRange(99901, 99976), itoa),
	)
	isbn10 = pbt.Custom("isbn10", func(t *pbt.T) string {
		g := group.Draw(t, "")
		rest := 9 - len(g)
		digits := pbt.StringOfN(pbt.SampledFrom([]rune("0123456789")), rest, rest).Draw(t, "")
		split := pbt.IntRange(0, rest).Draw(t, "")
		check := pbt.SampledFrom([]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "X"}).Draw(t, "")
		return g + "-" + digits[:split] + "-" + digits[split:] + "-" + check
	})
	isbn13 = pbt.Custom("isbn13", func(t *pbt.T) string {
		return pbt.SampledFrom([]string{"978", "979"}).Draw(t, "") + "-" + isbn10.Draw(t, "")
	})
	isbn = pbt.OneOf(isbn13, isbn10)

	// friendlyUnicode excludes the zero code point, which the chapter's
	// driver could not send, and the LIKE wildcards, which would make the
	// model's substring check wrong.
	friendlyUnicode = pbt.StringOf(pbt.Filter(pbt.Rune(), func(r rune) bool { return r != 0 && r != '_' && r != '%' }))
	title           = friendlyUnicode
	author          = friendlyUnicode
)

func itoa(n int) string { return fmt.Sprint(n) }

// partial picks a substring of an existing value, which is what the search
// commands must find.
func partial(s string) pbt.Gen[string] {
	rs := []rune(s)
	return pbt.Custom("partial", func(t *pbt.T) string {
		start := pbt.IntRange(0, len(rs)).Draw(t, "")
		n := pbt.IntRange(0, len(rs)-start).Draw(t, "")
		return string(rs[start : start+n])
	})
}

func (m model) matches(field func(Book) string, term string) bool {
	for _, b := range m {
		if strings.Contains(field(b), term) {
			return true
		}
	}
	return false
}

func byAuthor(b Book) string { return b.Author }
func byTitle(b Book) string  { return b.Title }

type cmd = pbt.Cmd[model, *Store]

func expectErr(want error) func(model, pbt.Env, any) error {
	return func(_ model, _ pbt.Env, got any) error {
		err, _ := got.(error)
		if want == nil && err != nil || want != nil && !errors.Is(err, want) {
			return fmt.Errorf("got %v, want %v", got, want)
		}
		return nil
	}
}

// commands follows the precise version of the chapter: the shim's intent
// (new, exists, invalid, avail, unavail, taken, full) is part of the
// command name, so the statistics show how often each path ran, and the
// preconditions keep every intent true even after shrinking removes
// earlier commands.
func commands(m model) pbt.Gen[pbt.Command[model, *Store]] {
	notIn := func(isbn string) func(model) bool {
		return func(m model) bool { _, ok := m[isbn]; return !ok }
	}
	base := []pbt.Weighted[pbt.Command[model, *Store]]{
		{Weight: 3, Gen: pbt.Custom("add_book:new", func(t *pbt.T) pbt.Command[model, *Store] {
			b := Book{ISBN: isbn.Draw(t, ""), Title: title.Draw(t, ""), Author: author.Draw(t, "")}
			b.Owned = pbt.IntRange(0, 5).Draw(t, "")
			b.Available = pbt.IntRange(0, b.Owned).Draw(t, "")
			return cmd{Name: "add_book:new", Args: []any{b.ISBN, b.Title, b.Author, b.Owned, b.Available},
				Pre:  notIn(b.ISBN),
				Run:  func(s *Store, _ pbt.Env) any { return s.AddBook(b.ISBN, b.Title, b.Author, b.Owned, b.Available) },
				Next: func(m model, _ pbt.Var) model { return m.with(b) },
				Post: expectErr(nil)}
		})},
		{Weight: 1, Gen: pbt.Map(isbn, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "add_copy:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(s *Store, _ pbt.Env) any { return s.AddCopy(i) }, Post: expectErr(ErrNotFound)}
		})},
		{Weight: 1, Gen: pbt.Map(isbn, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "borrow_copy:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(s *Store, _ pbt.Env) any { return s.BorrowCopy(i) }, Post: expectErr(ErrNotFound)}
		})},
		{Weight: 1, Gen: pbt.Map(isbn, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "return_copy:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(s *Store, _ pbt.Env) any { return s.ReturnCopy(i) }, Post: expectErr(ErrNotFound)}
		})},
		{Weight: 1, Gen: pbt.Map(isbn, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "find_by_isbn:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(s *Store, _ pbt.Env) any { return result(s.FindByISBN(i)) }, Post: expectRows(nil)}
		})},
		{Weight: 1, Gen: pbt.Map(pbt.StringOfN(friendlyRune, 1, -1), func(term string) pbt.Command[model, *Store] {
			return cmd{Name: "find_by_author:invalid", Args: []any{term},
				Pre: func(m model) bool { return !m.matches(byAuthor, term) },
				Run: func(s *Store, _ pbt.Env) any { return result(s.FindByAuthor(term)) }, Post: expectRows(nil)}
		})},
		{Weight: 1, Gen: pbt.Map(pbt.StringOfN(friendlyRune, 1, -1), func(term string) pbt.Command[model, *Store] {
			return cmd{Name: "find_by_title:invalid", Args: []any{term},
				Pre: func(m model) bool { return !m.matches(byTitle, term) },
				Run: func(s *Store, _ pbt.Env) any { return result(s.FindByTitle(term)) }, Post: expectRows(nil)}
		})},
	}
	if len(m) == 0 {
		return pbt.Frequency(base...)
	}
	known := pbt.SampledFrom(m.isbns())
	in := func(i string) func(model) bool {
		return func(m model) bool { _, ok := m[i]; return ok }
	}
	stateful := []pbt.Weighted[pbt.Command[model, *Store]]{
		{Weight: 1, Gen: pbt.Map(known, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "add_book:exists", Args: []any{i}, Pre: in(i),
				Run:  func(s *Store, _ pbt.Env) any { return s.AddBook(i, "dup", "dup", 1, 1) },
				Post: expectErr(ErrDuplicate)}
		})},
		{Weight: 2, Gen: pbt.Map(known, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "add_copy:exists", Args: []any{i}, Pre: in(i),
				Run:  func(s *Store, _ pbt.Env) any { return s.AddCopy(i) },
				Next: func(m model, _ pbt.Var) model { b := m[i]; b.Owned++; b.Available++; return m.with(b) },
				Post: expectErr(nil)}
		})},
		{Weight: 2, Gen: pbt.Map(known, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "borrow_copy:avail", Args: []any{i},
				Pre:  func(m model) bool { b, ok := m[i]; return ok && b.Available > 0 },
				Run:  func(s *Store, _ pbt.Env) any { return s.BorrowCopy(i) },
				Next: func(m model, _ pbt.Var) model { b := m[i]; b.Available--; return m.with(b) },
				Post: expectErr(nil)}
		})},
		{Weight: 1, Gen: pbt.Map(known, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "borrow_copy:unavail", Args: []any{i},
				Pre:  func(m model) bool { b, ok := m[i]; return ok && b.Available == 0 },
				Run:  func(s *Store, _ pbt.Env) any { return s.BorrowCopy(i) },
				Post: expectErr(ErrUnavailable)}
		})},
		{Weight: 2, Gen: pbt.Map(known, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "return_copy:taken", Args: []any{i},
				Pre:  func(m model) bool { b, ok := m[i]; return ok && b.Owned > b.Available },
				Run:  func(s *Store, _ pbt.Env) any { return s.ReturnCopy(i) },
				Next: func(m model, _ pbt.Var) model { b := m[i]; b.Available++; return m.with(b) },
				Post: expectErr(nil)}
		})},
		{Weight: 1, Gen: pbt.Map(known, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "return_copy:full", Args: []any{i},
				Pre:  func(m model) bool { b, ok := m[i]; return ok && b.Owned == b.Available },
				Run:  func(s *Store, _ pbt.Env) any { return s.ReturnCopy(i) },
				Post: expectErr(ErrAllReturned)}
		})},
		{Weight: 2, Gen: pbt.Map(known, func(i string) pbt.Command[model, *Store] {
			return cmd{Name: "find_by_isbn:exists", Args: []any{i}, Pre: in(i),
				Run:  func(s *Store, _ pbt.Env) any { return result(s.FindByISBN(i)) },
				Post: func(m model, _ pbt.Env, got any) error { return expectRows([]Book{m[i]})(m, pbt.Env{}, got) }}
		})},
		{Weight: 2, Gen: pbt.Bind(known, func(i string) pbt.Gen[pbt.Command[model, *Store]] {
			return pbt.Map(partial(m[i].Author), func(term string) pbt.Command[model, *Store] {
				return cmd{Name: "find_by_author:exists", Args: []any{term},
					Pre:  func(m model) bool { return m.matches(byAuthor, term) },
					Run:  func(s *Store, _ pbt.Env) any { return result(s.FindByAuthor(term)) },
					Post: expectMatches(byAuthor, term)}
			})
		})},
		{Weight: 2, Gen: pbt.Bind(known, func(i string) pbt.Gen[pbt.Command[model, *Store]] {
			return pbt.Map(partial(m[i].Title), func(term string) pbt.Command[model, *Store] {
				return cmd{Name: "find_by_title:exists", Args: []any{term},
					Pre:  func(m model) bool { return m.matches(byTitle, term) },
					Run:  func(s *Store, _ pbt.Env) any { return result(s.FindByTitle(term)) },
					Post: expectMatches(byTitle, term)}
			})
		})},
	}
	return pbt.Frequency(append(base, stateful...)...)
}

var friendlyRune = pbt.Filter(pbt.Rune(), func(r rune) bool { return r != 0 && r != '_' && r != '%' })

// rows is the result of a search: the books or the error.
type rows struct {
	books []Book
	err   error
}

func result(books []Book, err error) rows { return rows{books, err} }

func expectRows(want []Book) func(model, pbt.Env, any) error {
	return func(_ model, _ pbt.Env, got any) error {
		r, ok := got.(rows)
		if !ok || r.err != nil {
			return fmt.Errorf("got %v, want rows", got)
		}
		if len(r.books) == 0 && len(want) == 0 {
			return nil
		}
		if !reflect.DeepEqual(r.books, want) {
			return fmt.Errorf("got %v, want %v", r.books, want)
		}
		return nil
	}
}

// expectMatches checks every returned row against the model and, beyond
// the chapter, that no matching row is missing.
func expectMatches(field func(Book) string, term string) func(model, pbt.Env, any) error {
	return func(m model, _ pbt.Env, got any) error {
		r, ok := got.(rows)
		if !ok || r.err != nil {
			return fmt.Errorf("got %v, want rows", got)
		}
		found := map[string]bool{}
		for _, b := range r.books {
			if !strings.Contains(field(b), term) {
				return fmt.Errorf("row %v does not contain %q", b, term)
			}
			if mb, ok := m[b.ISBN]; !ok || mb != b {
				return fmt.Errorf("row %v differs from the model's %v", b, mb)
			}
			found[b.ISBN] = true
		}
		for isbn, b := range m {
			if strings.Contains(field(b), term) && !found[isbn] {
				return fmt.Errorf("row %v matches %q but was not returned", b, term)
			}
		}
		return nil
	}
}

func openStore(t *pbt.T) *Store {
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBookstore(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, openStore(t))
	})
}

func TestBookstoreParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(model{}, commands).Draw(t, "cmds")
		prog.RunParallel(t, openStore(t))
	}, pbt.Checks(30))
}
