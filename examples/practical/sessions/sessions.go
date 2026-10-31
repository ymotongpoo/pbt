// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package sessions is a practical example of a state machine test with
// time. A session store issues tokens that expire after a period of
// inactivity, are extended by a refresh and are revoked by a logout. The
// test drives one session through the states anonymous, active, expired
// and revoked with the fsm package, advances a fake clock, and checks
// that every operation answers as the state demands.
package sessions

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Token identifies a session.
type Token string

var (
	// ErrUnknown is returned for a token the store never issued.
	ErrUnknown = errors.New("sessions: unknown token")
	// ErrExpired is returned for a session whose time to live ran out.
	ErrExpired = errors.New("sessions: session expired")
	// ErrRevoked is returned for a session that was logged out.
	ErrRevoked = errors.New("sessions: session revoked")
)

// Options select the planted bug of the example. The zero value is the
// correct store.
type Options struct {
	// ReviveOnRefresh makes Refresh extend an expired session instead of
	// rejecting it, so a token that timed out comes back to life.
	ReviveOnRefresh bool
}

// Store issues and checks sessions.
type Store struct {
	mu   sync.Mutex
	now  func() time.Time
	ttl  time.Duration
	seq  int
	sess map[Token]*session
	opts Options
}

type session struct {
	user    string
	expires time.Time
	revoked bool
}

// New returns a store whose sessions live for ttl after the login or the
// last refresh. now is the time source.
func New(now func() time.Time, ttl time.Duration, opts Options) *Store {
	return &Store{now: now, ttl: ttl, sess: map[Token]*session{}, opts: opts}
}

// Login starts a session for user and returns its token.
func (s *Store) Login(user string) Token {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	tok := Token(fmt.Sprintf("s%d", s.seq))
	s.sess[tok] = &session{user: user, expires: s.now().Add(s.ttl)}
	return tok
}

// lookup returns the session of a token that is still usable.
func (s *Store) lookup(tok Token) (*session, error) {
	se, ok := s.sess[tok]
	switch {
	case !ok:
		return nil, ErrUnknown
	case se.revoked:
		return nil, ErrRevoked
	case !s.now().Before(se.expires):
		return nil, ErrExpired
	}
	return se, nil
}

// Use returns the user of a usable session.
func (s *Store) Use(tok Token) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	se, err := s.lookup(tok)
	if err != nil {
		return "", err
	}
	return se.user, nil
}

// Refresh extends a usable session by the time to live.
func (s *Store) Refresh(tok Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	se, err := s.lookup(tok)
	if errors.Is(err, ErrExpired) && s.opts.ReviveOnRefresh {
		se, err = s.sess[tok], nil
	}
	if err != nil {
		return err
	}
	se.expires = s.now().Add(s.ttl)
	return nil
}

// Logout revokes a usable session.
func (s *Store) Logout(tok Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	se, err := s.lookup(tok)
	if err != nil {
		return err
	}
	se.revoked = true
	return nil
}
