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

// model is the notes that exist and the Vars of those that were deleted,
// which the commands keep presenting to check that they stay gone.
type model struct {
	live []note
	gone []pbt.Var
}

func (m model) find(v pbt.Var) (note, bool) {
	for _, n := range m.live {
		if n.id == v {
			return n, true
		}
	}
	return note{}, false
}

func (m model) knows(v pbt.Var) bool {
	_, live := m.find(v)
	return live || slices.Contains(m.gone, v)
}

// all lists every Var the model knows, live first.
func (m model) all() []pbt.Var {
	all := make([]pbt.Var, 0, len(m.live)+len(m.gone))
	for _, n := range m.live {
		all = append(all, n.id)
	}
	return append(all, m.gone...)
}

func (m model) put(n note) model {
	live := slices.Clone(m.live)
	for i := range live {
		if live[i].id == n.id {
			live[i] = n
			return model{live, m.gone}
		}
	}
	return model{append(live, n), m.gone}
}

func (m model) remove(v pbt.Var) model {
	live := slices.DeleteFunc(slices.Clone(m.live), func(n note) bool { return n.id == v })
	return model{live, append(slices.Clone(m.gone), v)}
}

type cmd = pbt.Cmd[model, *Client]
type command = pbt.Command[model, *Client]

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

// expect checks the status of a response and, when a note is wanted, its
// fields.
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

var create = pbt.Map(title, func(title string) command {
	return cmd{Name: "create", Args: []any{title},
		Run:  func(c *Client, _ pbt.Env) any { return call(func() (Response, error) { return c.Create(title) }) },
		Next: func(m model, v pbt.Var) model { return m.put(note{id: v, title: title, version: 1}) },
		Post: func(_ model, _ pbt.Env, got any) error {
			if err := expectResponse(got, http.StatusCreated, nil); err != nil {
				return err
			}
			if n := got.(Response).Note; n.ID == "" || n.Title != title || n.Version != 1 {
				return fmt.Errorf("created %+v, want title %q at version 1", n, title)
			}
			return nil
		}}
})

func get(m model) pbt.Gen[command] {
	return pbt.Map(pbt.SampledFrom(m.all()), func(v pbt.Var) command {
		return cmd{Name: "get", Args: []any{v},
			Pre: func(m model) bool { return m.knows(v) },
			Run: func(c *Client, env pbt.Env) any { return call(func() (Response, error) { return c.Get(idOf(env, v)) }) },
			Post: func(m model, env pbt.Env, got any) error {
				n, live := m.find(v)
				if !live {
					return expectResponse(got, http.StatusNotFound, nil)
				}
				return expectResponse(got, http.StatusOK, &Note{idOf(env, v), n.title, n.version})
			}}
	})
}

// update sends the version the client believes the note is at. The
// generator sends a stale one some of the time, and the model decides
// from the version it sends whether the server must accept it.
func update(m model) pbt.Gen[command] {
	return pbt.Custom("update", func(t *pbt.T) command {
		v := pbt.SampledFrom(m.all()).Draw(t, "")
		title := title.Draw(t, "")
		n, _ := m.find(v)
		version := n.version
		if n.version > 1 && pbt.Bool().Draw(t, "") {
			version--
		}
		return cmd{Name: "update", Args: []any{v, title, version},
			Pre: func(m model) bool { return m.knows(v) },
			Run: func(c *Client, env pbt.Env) any {
				return call(func() (Response, error) { return c.Update(idOf(env, v), title, version) })
			},
			Next: func(m model, _ pbt.Var) model {
				if n, live := m.find(v); live && n.version == version {
					return m.put(note{id: v, title: title, version: version + 1})
				}
				return m
			},
			Post: func(m model, env pbt.Env, got any) error {
				n, live := m.find(v)
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

func remove(m model) pbt.Gen[command] {
	return pbt.Map(pbt.SampledFrom(m.all()), func(v pbt.Var) command {
		return cmd{Name: "delete", Args: []any{v},
			Pre: func(m model) bool { return m.knows(v) },
			Run: func(c *Client, env pbt.Env) any {
				return call(func() (Response, error) { return c.Delete(idOf(env, v)) })
			},
			Next: func(m model, _ pbt.Var) model {
				if _, live := m.find(v); live {
					return m.remove(v)
				}
				return m
			},
			Post: func(m model, _ pbt.Env, got any) error {
				if _, live := m.find(v); live {
					return expectResponse(got, http.StatusNoContent, nil)
				}
				return expectResponse(got, http.StatusNotFound, nil)
			}}
	})
}

var list = pbt.Just[command](cmd{Name: "list",
	Run: func(c *Client, _ pbt.Env) any { return call(func() (Response, error) { return c.List() }) },
	Post: func(m model, env pbt.Env, got any) error {
		if err := expectResponse(got, http.StatusOK, nil); err != nil {
			return err
		}
		want := make([]Note, 0, len(m.live))
		for _, n := range m.live {
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

func commands(m model) pbt.Gen[command] {
	alts := []pbt.Weighted[command]{{Weight: 3, Gen: create}, {Weight: 1, Gen: list}}
	if len(m.all()) > 0 {
		alts = append(alts,
			pbt.Weighted[command]{Weight: 3, Gen: get(m)},
			pbt.Weighted[command]{Weight: 3, Gen: update(m)},
			pbt.Weighted[command]{Weight: 1, Gen: remove(m)},
		)
	}
	return pbt.Frequency(alts...)
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
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, serve(t, Options{}))
	})
}

// A server that ignores If-Match accepts an update from a stale version.
// The shortest program creates a note, updates it once, so that a stale
// version exists, and updates it again with the old version.
func TestStaleUpdateIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, serve(t, Options{IgnoreIfMatch: true}))
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "\n") > 4 || strings.Count(program, "= update(") != 2 || !strings.Contains(report, "status 200, want 412") {
		t.Errorf("expected a create and two updates, the second one stale:\n%s", report)
	}
}

func TestNotesAPIParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(model{}, commands).Draw(t, "cmds")
		prog.RunParallel(t, serve(t, Options{}))
	})
}

// Two concurrent creates on the racy server get the same id, and the
// second overwrites the first. A get or an update of the first note then
// returns the second note's title, which no interleaving explains.
func TestRacyIDsAreCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(model{}, commands).Draw(t, "cmds")
		for range 3 {
			prog.RunParallel(t, serve(t, Options{RacyIDs: true}))
		}
	}, pbt.Seed(1), pbt.Checks(200))
	if !strings.Contains(report, "no interleaving") {
		t.Errorf("expected a failure that no interleaving explains:\n%s", report)
	}
}
