// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package httpapi is a practical example of a stateful test of an HTTP
// API. The server keeps notes with a version each; a client creates,
// reads, updates, deletes and lists them over HTTP, and an update must
// carry the version it saw in If-Match, so that two clients cannot
// overwrite each other. The test talks to a real server from
// net/http/httptest, keeps the server-assigned ids as Vars, checks status
// codes and bodies, and runs the same commands in parallel to catch an id
// allocated outside the lock.
package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Note is a resource of the API.
type Note struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version int    `json:"version"`
}

// Options select the planted bugs of the example. The zero value is the
// correct server.
type Options struct {
	// IgnoreIfMatch makes PUT skip the comparison of If-Match with the
	// current version, so a client working from a stale copy overwrites a
	// newer one.
	IgnoreIfMatch bool
	// RacyIDs allocates the id of a new note outside the lock that guards
	// the notes, so two concurrent creates can get the same id.
	RacyIDs bool
}

type server struct {
	mu    sync.Mutex
	next  int
	notes map[string]Note
	opts  Options
}

// NewHandler returns the API: POST and GET /notes, GET, PUT and DELETE
// /notes/{id}.
func NewHandler(opts Options) http.Handler {
	s := &server{notes: map[string]Note{}, opts: opts}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /notes", s.create)
	mux.HandleFunc("GET /notes", s.list)
	mux.HandleFunc("GET /notes/{id}", s.get)
	mux.HandleFunc("PUT /notes/{id}", s.update)
	mux.HandleFunc("DELETE /notes/{id}", s.delete)
	return mux
}

type noteInput struct {
	Title string `json:"title"`
}

func readInput(w http.ResponseWriter, r *http.Request) (noteInput, bool) {
	var in noteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Title == "" {
		http.Error(w, "a JSON body with a non-empty title is required", http.StatusBadRequest)
		return in, false
	}
	return in, true
}

func etag(n Note) string { return strconv.Quote(strconv.Itoa(n.Version)) }

func writeNote(w http.ResponseWriter, status int, n Note) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag(n))
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(n)
}

// allocate returns the id of a new note.
func (s *server) allocate() string {
	s.mu.Lock()
	if !s.opts.RacyIDs {
		defer s.mu.Unlock()
		s.next++
		return fmt.Sprintf("n%d", s.next)
	}
	// The planted race: the counter is read under the lock and written
	// back under it, but another create can run in between and read the
	// same value.
	id := s.next + 1
	s.mu.Unlock()
	time.Sleep(time.Millisecond)
	s.mu.Lock()
	s.next = id
	s.mu.Unlock()
	return fmt.Sprintf("n%d", id)
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	in, ok := readInput(w, r)
	if !ok {
		return
	}
	n := Note{ID: s.allocate(), Title: in.Title, Version: 1}
	s.mu.Lock()
	s.notes[n.ID] = n
	s.mu.Unlock()
	w.Header().Set("Location", "/notes/"+n.ID)
	writeNote(w, http.StatusCreated, n)
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	n, ok := s.notes[r.PathValue("id")]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeNote(w, http.StatusOK, n)
}

func (s *server) update(w http.ResponseWriter, r *http.Request) {
	in, ok := readInput(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[r.PathValue("id")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	match := r.Header.Get("If-Match")
	if match == "" {
		http.Error(w, "If-Match is required", http.StatusPreconditionRequired)
		return
	}
	if match != etag(n) && !s.opts.IgnoreIfMatch {
		http.Error(w, "the note changed since it was read", http.StatusPreconditionFailed)
		return
	}
	n.Title = in.Title
	n.Version++
	s.notes[n.ID] = n
	writeNote(w, http.StatusOK, n)
}

func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := r.PathValue("id")
	if _, ok := s.notes[id]; !ok {
		http.NotFound(w, r)
		return
	}
	delete(s.notes, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	notes := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		notes = append(notes, n)
	}
	s.mu.Unlock()
	sort.Slice(notes, func(i, j int) bool { return seq(notes[i].ID) < seq(notes[j].ID) })
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

func seq(id string) int {
	n, _ := strconv.Atoi(id[1:])
	return n
}
