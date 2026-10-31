// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Response is what a call returned: the status, and the note or notes
// when the status is a success.
type Response struct {
	Status int
	Note   Note
	Notes  []Note
}

// GoString keeps the history short: the status and the note.
func (r Response) GoString() string {
	switch {
	case r.Notes != nil:
		return fmt.Sprintf("%d %v", r.Status, r.Notes)
	case r.Status >= 200 && r.Status < 300 && r.Note.ID != "":
		return fmt.Sprintf("%d %v", r.Status, r.Note)
	}
	return strconv.Itoa(r.Status)
}

// Client calls the API.
type Client struct {
	base string
	http *http.Client
}

// NewClient returns a client of the API served at base.
func NewClient(base string, hc *http.Client) *Client {
	return &Client{base: base, http: hc}
}

// Create adds a note.
func (c *Client) Create(title string) (Response, error) {
	return c.do(http.MethodPost, "/notes", noteInput{title}, "", true)
}

// Get reads a note.
func (c *Client) Get(id string) (Response, error) {
	return c.do(http.MethodGet, "/notes/"+id, nil, "", true)
}

// Update changes the title of the note, provided it is still at version.
func (c *Client) Update(id, title string, version int) (Response, error) {
	return c.do(http.MethodPut, "/notes/"+id, noteInput{title}, strconv.Quote(strconv.Itoa(version)), true)
}

// Delete removes a note.
func (c *Client) Delete(id string) (Response, error) {
	return c.do(http.MethodDelete, "/notes/"+id, nil, "", false)
}

// List returns every note.
func (c *Client) List() (Response, error) {
	var notes []Note
	resp, err := c.call(http.MethodGet, "/notes", nil, "", &notes)
	if notes == nil {
		notes = []Note{}
	}
	resp.Notes = notes
	return resp, err
}

func (c *Client) do(method, path string, body any, ifMatch string, note bool) (Response, error) {
	var n Note
	resp, err := c.call(method, path, body, ifMatch, &n)
	if note {
		resp.Note = n
	}
	return resp, err
}

// call sends one request and decodes a successful JSON body into out.
// A transport failure is an error; an HTTP error status is a Response.
func (c *Client) call(method, path string, body any, ifMatch string, out any) (Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return Response{}, err
		}
	}
	req, err := http.NewRequest(method, c.base+path, &buf)
	if err != nil {
		return Response{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	r := Response{Status: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || resp.StatusCode == http.StatusNoContent {
		io.Copy(io.Discard, resp.Body)
		return r, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return r, fmt.Errorf("%s %s: decoding the body: %w", method, path, err)
	}
	return r, nil
}
