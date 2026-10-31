// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

// note is what the test knows about one note: the Var of the response
// that created it, which carries the id the server chose, and the title
// and version it must have now.
type note struct {
	id      pbt.Var
	title   string
	version int
}

// harness holds the client of the server under test and the model: the
// notes that exist and the Vars of those that were deleted, which the
// commands keep presenting to check that they stay gone. A test replaces
// the client to run one program against a fresh server.
type harness struct {
	client *Client
	live   []note
	gone   []pbt.Var
}

func (h *harness) reset() { h.live, h.gone = nil, nil }

func (h *harness) find(v pbt.Var) (note, bool) {
	for _, n := range h.live {
		if n.id == v {
			return n, true
		}
	}
	return note{}, false
}

func (h *harness) knows(v pbt.Var) bool {
	_, live := h.find(v)
	return live || slices.Contains(h.gone, v)
}

// all lists every Var the model knows, live first.
func (h *harness) all() []pbt.Var {
	all := make([]pbt.Var, 0, len(h.live)+len(h.gone))
	for _, n := range h.live {
		all = append(all, n.id)
	}
	return append(all, h.gone...)
}

func (h *harness) put(n note) {
	for i := range h.live {
		if h.live[i].id == n.id {
			h.live[i] = n
			return
		}
	}
	h.live = append(h.live, n)
}

func (h *harness) remove(v pbt.Var) {
	h.live = slices.DeleteFunc(h.live, func(n note) bool { return n.id == v })
	h.gone = append(h.gone, v)
}

// idOf is the id the server gave the note created by the response v.
func idOf(env pbt.Env, v pbt.Var) string { return pbt.Resolve[Response](env, v).Note.ID }

// call runs one request. A transport failure becomes the result, which
// no postcondition accepts, so it fails the case with its message.
func call(f func() (Response, error)) any {
	r, err := f()
	if err != nil {
		return err
	}
	return r
}

// expectResponse checks the status of a response and, when a note is
// wanted, its fields.
func expectResponse(got any, status int, want *Note) error {
	r, ok := got.(Response)
	if !ok {
		return fmt.Errorf("the request failed: %v", got)
	}
	if r.Status != status {
		return fmt.Errorf("status %d, want %d", r.Status, status)
	}
	if want != nil && r.Note != *want {
		return fmt.Errorf("note %+v, want %+v", r.Note, *want)
	}
	return nil
}

var title = pbt.Map(pbt.IntRange(1, 999), func(n int) string { return fmt.Sprintf("note %d", n) })

func (h *harness) create() pbt.Gen[pbt.Command] {
	return pbt.Map(title, func(title string) pbt.Command {
		return pbt.Command{Name: "create", Args: []any{title},
			Run:  func(pbt.Env) any { return call(func() (Response, error) { return h.client.Create(title) }) },
			Next: func(v pbt.Var) { h.put(note{id: v, title: title, version: 1}) },
			Post: func(_ pbt.Env, got any) error {
				if err := expectResponse(got, http.StatusCreated, nil); err != nil {
					return err
				}
				if n := got.(Response).Note; n.ID == "" || n.Title != title || n.Version != 1 {
					return fmt.Errorf("created %+v, want title %q at version 1", n, title)
				}
				return nil
			}}
	})
}

func (h *harness) get() pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(h.all()), func(v pbt.Var) pbt.Command {
		return pbt.Command{Name: "get", Args: []any{v},
			Pre: func() bool { return h.knows(v) },
			Run: func(env pbt.Env) any { return call(func() (Response, error) { return h.client.Get(idOf(env, v)) }) },
			Post: func(env pbt.Env, got any) error {
				n, live := h.find(v)
				if !live {
					return expectResponse(got, http.StatusNotFound, nil)
				}
				return expectResponse(got, http.StatusOK, &Note{idOf(env, v), n.title, n.version})
			}}
	})
}

// update sends the version the client believes the note is at. The
// generator sends a stale one some of the time, and the postcondition
// decides from the version it sent and the model's current version
// whether the server must accept it, so the command stays valid after the
// shrinker removes the commands around it.
func (h *harness) update() pbt.Gen[pbt.Command] {
	return pbt.Custom("update", func(t *pbt.T) pbt.Command {
		v := pbt.SampledFrom(h.all()).Draw(t, "")
		title := title.Draw(t, "")
		n, _ := h.find(v)
		version := n.version
		if n.version > 1 && pbt.Bool().Draw(t, "") {
			version--
		}
		return pbt.Command{Name: "update", Args: []any{v, title, version},
			Pre: func() bool { return h.knows(v) },
			Run: func(env pbt.Env) any {
				return call(func() (Response, error) { return h.client.Update(idOf(env, v), title, version) })
			},
			Next: func(pbt.Var) {
				if n, live := h.find(v); live && n.version == version {
					h.put(note{id: v, title: title, version: version + 1})
				}
			},
			Post: func(env pbt.Env, got any) error {
				n, live := h.find(v)
				switch {
				case !live:
					return expectResponse(got, http.StatusNotFound, nil)
				case n.version != version:
					return expectResponse(got, http.StatusPreconditionFailed, nil)
				}
				return expectResponse(got, http.StatusOK, &Note{idOf(env, v), title, version + 1})
			}}
	})
}

func (h *harness) delete() pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(h.all()), func(v pbt.Var) pbt.Command {
		return pbt.Command{Name: "delete", Args: []any{v},
			Pre: func() bool { return h.knows(v) },
			Run: func(env pbt.Env) any { return call(func() (Response, error) { return h.client.Delete(idOf(env, v)) }) },
			Next: func(pbt.Var) {
				if _, live := h.find(v); live {
					h.remove(v)
				}
			},
			Post: func(_ pbt.Env, got any) error {
				if _, live := h.find(v); live {
					return expectResponse(got, http.StatusNoContent, nil)
				}
				return expectResponse(got, http.StatusNotFound, nil)
			}}
	})
}

func (h *harness) list() pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "list",
		Run: func(pbt.Env) any { return call(func() (Response, error) { return h.client.List() }) },
		Post: func(env pbt.Env, got any) error {
			if err := expectResponse(got, http.StatusOK, nil); err != nil {
				return err
			}
			want := make([]Note, 0, len(h.live))
			for _, n := range h.live {
				want = append(want, Note{idOf(env, n.id), n.title, n.version})
			}
			byID := func(a, b Note) int { return seq(a.ID) - seq(b.ID) }
			slices.SortFunc(want, byID)
			notes := slices.SortedFunc(slices.Values(got.(Response).Notes), byID)
			if !slices.Equal(notes, want) {
				return fmt.Errorf("list returned %v, want %v", notes, want)
			}
			return nil
		}})
}

func (h *harness) commands() pbt.Gen[pbt.Command] {
	alts := []pbt.Weighted[pbt.Command]{pbt.W(3, h.create()), pbt.W(1, h.list())}
	if len(h.all()) > 0 {
		alts = append(alts, pbt.W(3, h.get()), pbt.W(3, h.update()), pbt.W(1, h.delete()))
	}
	return pbt.Frequency(alts...)
}

func (h *harness) program() pbt.Gen[pbt.Program] {
	return pbt.Commands(h.reset, h.commands, pbt.ModelState(func() any { return h.live }))
}

// serve starts a fresh server for one test case and returns a client of
// it. The server is closed when the case ends.
func serve(t *pbt.T, opts Options) *Client {
	srv := httptest.NewServer(NewHandler(opts))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, srv.Client())
}

func TestNotesAPI(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{client: serve(t, Options{})}
		h.program().Draw(t, "cmds").RunSequential(t)
	})
}

// A server that ignores If-Match accepts an update from a stale version.
// The shortest program creates a note, updates it once, so that a stale
// version exists, and updates it again with the old version.
func TestStaleUpdateIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		h := &harness{client: serve(t, Options{IgnoreIfMatch: true})}
		h.program().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "\n") > 4 || strings.Count(program, "= update(") != 2 || !strings.Contains(report, "status 200, want 412") {
		t.Errorf("expected a create and two updates, the second one stale:\n%s", report)
	}
}

func TestNotesAPIParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{client: serve(t, Options{})}
		pbt.ParallelCommands(h.reset, h.commands).Draw(t, "cmds").RunParallel(t)
	})
}

// Two concurrent creates on the racy server get the same id, and the
// second overwrites the first. A get or a list after them then shows one
// note where the model has two, which no interleaving explains.
func TestRacyIDsAreCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		h := &harness{}
		prog := pbt.ParallelCommands(h.reset, h.commands).Draw(t, "cmds")
		for range 3 {
			h.client = serve(t, Options{RacyIDs: true})
			prog.RunParallel(t)
		}
	}, pbt.Seed(1), pbt.Checks(200))
	if !strings.Contains(report, "no interleaving") {
		t.Errorf("expected a failure that no interleaving explains:\n%s", report)
	}
}
