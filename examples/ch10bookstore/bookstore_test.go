// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch10bookstore

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// harness holds the store under test and the model: the row the store
// should hold for each ISBN. The commands are closures over it.
type harness struct {
	store *Store
	books map[string]Book
}

func (h *harness) reset() { h.books = map[string]Book{} }

func (h *harness) isbns() []string { return slices.Sorted(maps.Keys(h.books)) }

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
	// model's substring check wrong. It is bounded to the 256 characters of
	// the varchar columns: PostgreSQL rejects longer values, SQLite does
	// not enforce the length.
	friendlyUnicode = pbt.StringOfN(pbt.Filter(pbt.Rune(), func(r rune) bool { return r != 0 && r != '_' && r != '%' }), 0, 256)
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

func (h *harness) matches(field func(Book) string, term string) bool {
	for _, b := range h.books {
		if strings.Contains(field(b), term) {
			return true
		}
	}
	return false
}

func byAuthor(b Book) string { return b.Author }
func byTitle(b Book) string  { return b.Title }

func expectErr(want error) func(pbt.Env, any) error {
	return func(_ pbt.Env, got any) error {
		err, _ := got.(error)
		if want == nil && err != nil || want != nil && !errors.Is(err, want) {
			return fmt.Errorf("got %v, want %v", got, want)
		}
		return nil
	}
}

// commands follows the precise version of the chapter: the shim's intent
// (new, exists, invalid, avail, unavail, taken, full) is part of the
// command name, so the statistics show how often each path ran. Pre filters
// candidates against the current model, including when shrinking regenerates
// commands after a deletion; a known ISBN alone does not ensure availability.
func (h *harness) commands() pbt.CommandGen {
	notIn := func(isbn string) func() bool {
		return func() bool { _, ok := h.books[isbn]; return !ok }
	}
	s := h.store
	base := []pbt.Weighted[pbt.Command]{
		pbt.W(3, pbt.Custom("add_book:new", func(t *pbt.T) pbt.Command {
			b := Book{ISBN: isbn.Draw(t, ""), Title: title.Draw(t, ""), Author: author.Draw(t, "")}
			b.Owned = pbt.IntRange(0, 5).Draw(t, "")
			b.Available = pbt.IntRange(0, b.Owned).Draw(t, "")
			return pbt.Command{Name: "add_book:new", Args: []any{b.ISBN, b.Title, b.Author, b.Owned, b.Available},
				Pre:  notIn(b.ISBN),
				Run:  func(pbt.Env) any { return s.AddBook(b.ISBN, b.Title, b.Author, b.Owned, b.Available) },
				Next: func(pbt.Var) { h.books[b.ISBN] = b },
				Post: expectErr(nil)}
		})),
		pbt.W(1, pbt.Map(isbn, func(i string) pbt.Command {
			return pbt.Command{Name: "add_copy:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(pbt.Env) any { return s.AddCopy(i) }, Post: expectErr(ErrNotFound)}
		})),
		pbt.W(1, pbt.Map(isbn, func(i string) pbt.Command {
			return pbt.Command{Name: "borrow_copy:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(pbt.Env) any { return s.BorrowCopy(i) }, Post: expectErr(ErrNotFound)}
		})),
		pbt.W(1, pbt.Map(isbn, func(i string) pbt.Command {
			return pbt.Command{Name: "return_copy:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(pbt.Env) any { return s.ReturnCopy(i) }, Post: expectErr(ErrNotFound)}
		})),
		pbt.W(1, pbt.Map(isbn, func(i string) pbt.Command {
			return pbt.Command{Name: "find_by_isbn:invalid", Args: []any{i}, Pre: notIn(i),
				Run: func(pbt.Env) any { return result(s.FindByISBN(i)) }, Post: expectRows(nil)}
		})),
		pbt.W(1, pbt.Map(pbt.StringOfN(friendlyRune, 1, -1), func(term string) pbt.Command {
			return pbt.Command{Name: "find_by_author:invalid", Args: []any{term},
				Pre: func() bool { return !h.matches(byAuthor, term) },
				Run: func(pbt.Env) any { return result(s.FindByAuthor(term)) }, Post: expectRows(nil)}
		})),
		pbt.W(1, pbt.Map(pbt.StringOfN(friendlyRune, 1, -1), func(term string) pbt.Command {
			return pbt.Command{Name: "find_by_title:invalid", Args: []any{term},
				Pre: func() bool { return !h.matches(byTitle, term) },
				Run: func(pbt.Env) any { return result(s.FindByTitle(term)) }, Post: expectRows(nil)}
		})),
	}
	if len(h.books) == 0 {
		return pbt.Frequency(base...)
	}
	known := pbt.SampledFrom(h.isbns())
	in := func(i string) func() bool {
		return func() bool { _, ok := h.books[i]; return ok }
	}
	stateful := []pbt.Weighted[pbt.Command]{
		pbt.W(1, pbt.Map(known, func(i string) pbt.Command {
			return pbt.Command{Name: "add_book:exists", Args: []any{i}, Pre: in(i),
				Run:  func(pbt.Env) any { return s.AddBook(i, "dup", "dup", 1, 1) },
				Post: expectErr(ErrDuplicate)}
		})),
		pbt.W(2, pbt.Map(known, func(i string) pbt.Command {
			return pbt.Command{Name: "add_copy:exists", Args: []any{i}, Pre: in(i),
				Run:  func(pbt.Env) any { return s.AddCopy(i) },
				Next: func(pbt.Var) { b := h.books[i]; b.Owned++; b.Available++; h.books[i] = b },
				Post: expectErr(nil)}
		})),
		pbt.W(2, pbt.Map(known, func(i string) pbt.Command {
			return pbt.Command{Name: "borrow_copy:avail", Args: []any{i},
				Pre:  func() bool { b, ok := h.books[i]; return ok && b.Available > 0 },
				Run:  func(pbt.Env) any { return s.BorrowCopy(i) },
				Next: func(pbt.Var) { b := h.books[i]; b.Available--; h.books[i] = b },
				Post: expectErr(nil)}
		})),
		pbt.W(1, pbt.Map(known, func(i string) pbt.Command {
			return pbt.Command{Name: "borrow_copy:unavail", Args: []any{i},
				Pre:  func() bool { b, ok := h.books[i]; return ok && b.Available == 0 },
				Run:  func(pbt.Env) any { return s.BorrowCopy(i) },
				Post: expectErr(ErrUnavailable)}
		})),
		pbt.W(2, pbt.Map(known, func(i string) pbt.Command {
			return pbt.Command{Name: "return_copy:taken", Args: []any{i},
				Pre:  func() bool { b, ok := h.books[i]; return ok && b.Owned > b.Available },
				Run:  func(pbt.Env) any { return s.ReturnCopy(i) },
				Next: func(pbt.Var) { b := h.books[i]; b.Available++; h.books[i] = b },
				Post: expectErr(nil)}
		})),
		pbt.W(1, pbt.Map(known, func(i string) pbt.Command {
			return pbt.Command{Name: "return_copy:full", Args: []any{i},
				Pre:  func() bool { b, ok := h.books[i]; return ok && b.Owned == b.Available },
				Run:  func(pbt.Env) any { return s.ReturnCopy(i) },
				Post: expectErr(ErrAllReturned)}
		})),
		pbt.W(2, pbt.Map(known, func(i string) pbt.Command {
			return pbt.Command{Name: "find_by_isbn:exists", Args: []any{i}, Pre: in(i),
				Run:  func(pbt.Env) any { return result(s.FindByISBN(i)) },
				Post: func(env pbt.Env, got any) error { return expectRows([]Book{h.books[i]})(env, got) }}
		})),
		pbt.W(2, pbt.Bind(known, func(i string) pbt.CommandGen {
			return pbt.Map(partial(h.books[i].Author), func(term string) pbt.Command {
				return pbt.Command{Name: "find_by_author:exists", Args: []any{term},
					Pre:  func() bool { return h.matches(byAuthor, term) },
					Run:  func(pbt.Env) any { return result(s.FindByAuthor(term)) },
					Post: h.expectMatches(byAuthor, term)}
			})
		})),
		pbt.W(2, pbt.Bind(known, func(i string) pbt.CommandGen {
			return pbt.Map(partial(h.books[i].Title), func(term string) pbt.Command {
				return pbt.Command{Name: "find_by_title:exists", Args: []any{term},
					Pre:  func() bool { return h.matches(byTitle, term) },
					Run:  func(pbt.Env) any { return result(s.FindByTitle(term)) },
					Post: h.expectMatches(byTitle, term)}
			})
		})),
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

func expectRows(want []Book) func(pbt.Env, any) error {
	return func(_ pbt.Env, got any) error {
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
func (h *harness) expectMatches(field func(Book) string, term string) func(pbt.Env, any) error {
	return func(_ pbt.Env, got any) error {
		m := h.books
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

// openStore opens a fresh store per test case: SQLite in memory, or the
// PostgreSQL database named by PBT_BOOKSTORE_DSN, for example
// postgres://postgres:secret@localhost:5432/crud_db.
func openStore(t *pbt.T) *Store {
	var s *Store
	var err error
	if dsn := os.Getenv("PBT_BOOKSTORE_DSN"); dsn != "" {
		s, err = OpenPostgres(dsn)
	} else {
		s, err = Open()
	}
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBookstore(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{store: openStore(t)}
		prog := pbt.Commands(h.reset, h.commands, pbt.ModelState(func() any { return h.books })).Draw(t, "cmds")
		prog.RunSequential(t)
	})
}

func TestBookstoreParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{store: openStore(t)}
		prog := pbt.ParallelCommands(h.reset, h.commands).Draw(t, "cmds")
		prog.RunParallel(t)
	}, pbt.Checks(30))
}
