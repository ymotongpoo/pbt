// Package ch10bookstore ports Chapter 10 of the PropEr Testing book: a
// bookstore inventory on a SQL database, tested as a state machine. The
// chapter runs on PostgreSQL; this port runs on the pure-Go SQLite driver so
// that go test needs nothing else. LIKE is set to case-sensitive matching so
// that the model's substring check and the database agree.
package ch10bookstore

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// Book is one row of the books table.
type Book struct {
	ISBN      string
	Title     string
	Author    string
	Owned     int
	Available int
}

var (
	// ErrNotFound reports an ISBN that is not in the store.
	ErrNotFound = errors.New("not found")
	// ErrUnavailable reports that every copy is borrowed.
	ErrUnavailable = errors.New("unavailable")
	// ErrAllReturned reports a return when no copy is out.
	ErrAllReturned = errors.New("no copy to return")
	// ErrDuplicate reports an ISBN that is already in the store.
	ErrDuplicate = errors.New("duplicate isbn")
)

// Store is the inventory. Open it with Open and close it with Close.
type Store struct{ db *sql.DB }

// Open creates a store on a fresh in-memory SQLite database with the books
// table. One connection is used, because every connection to an in-memory
// SQLite database would see its own database.
func Open() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	for _, stmt := range []string{
		"PRAGMA case_sensitive_like = ON",
		`CREATE TABLE books (
			isbn varchar(20) PRIMARY KEY,
			title varchar(256) NOT NULL,
			author varchar(256) NOT NULL,
			owned smallint DEFAULT 0,
			available smallint DEFAULT 0)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}

// Close drops the database.
func (s *Store) Close() error { return s.db.Close() }

// AddBook inserts a book with the given number of copies.
func (s *Store) AddBook(isbn, title, author string, owned, available int) error {
	_, err := s.db.Exec("INSERT INTO books (isbn, title, author, owned, available) VALUES (?, ?, ?, ?, ?)",
		isbn, title, author, owned, available)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDuplicate, err)
	}
	return nil
}

// AddCopy adds one copy of a book.
func (s *Store) AddCopy(isbn string) error {
	return s.single("UPDATE books SET owned = owned + 1, available = available + 1 WHERE isbn = ?", ErrNotFound, isbn)
}

// BorrowCopy takes one copy out. The chapter's first version reported an
// unavailable book as not found; checking the ISBN first tells the two
// apart.
func (s *Store) BorrowCopy(isbn string) error {
	if err := s.exists(isbn); err != nil {
		return err
	}
	return s.single("UPDATE books SET available = available - 1 WHERE isbn = ? AND available > 0", ErrUnavailable, isbn)
}

// ReturnCopy brings one copy back. The chapter's first version let the
// available count exceed the owned count; the condition in the query is
// the fix.
func (s *Store) ReturnCopy(isbn string) error {
	if err := s.exists(isbn); err != nil {
		return err
	}
	return s.single("UPDATE books SET available = available + 1 WHERE isbn = ? AND available < owned", ErrAllReturned, isbn)
}

func (s *Store) exists(isbn string) error {
	rows, err := s.FindByISBN(isbn)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) single(query string, zeroRows error, args ...any) error {
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return zeroRows
	}
	return nil
}

// FindByAuthor returns the books whose author contains the term.
func (s *Store) FindByAuthor(term string) ([]Book, error) {
	return s.query("SELECT isbn, title, author, owned, available FROM books WHERE author LIKE ?", "%"+term+"%")
}

// FindByTitle returns the books whose title contains the term.
func (s *Store) FindByTitle(term string) ([]Book, error) {
	return s.query("SELECT isbn, title, author, owned, available FROM books WHERE title LIKE ?", "%"+term+"%")
}

// FindByISBN returns the book with the ISBN, if any.
func (s *Store) FindByISBN(isbn string) ([]Book, error) {
	return s.query("SELECT isbn, title, author, owned, available FROM books WHERE isbn = ?", isbn)
}

func (s *Store) query(q string, args ...any) ([]Book, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var books []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ISBN, &b.Title, &b.Author, &b.Owned, &b.Available); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
