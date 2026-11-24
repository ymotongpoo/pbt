// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// The productions of RFC 3986 as regular expressions, written from the
// ABNF of Appendix A independently of the generators.
const (
	reUnres   = `[A-Za-z0-9\-._~]`
	rePct     = `%[0-9A-Fa-f]{2}`
	reSub     = `[!$&'()*+,;=]`
	rePchar   = `(?:` + reUnres + `|` + rePct + `|` + reSub + `|[:@])`
	reSegment = rePchar + `*`
	reSegNZ   = rePchar + `+`
	reSegNZNC = `(?:` + reUnres + `|` + rePct + `|` + reSub + `|@)+`
	reQuery   = `(?:` + rePchar + `|[/?])*`
	reScheme  = `[A-Za-z][A-Za-z0-9+\-.]*`
	reUser    = `(?:` + reUnres + `|` + rePct + `|` + reSub + `|:)*`
	reHost    = `(?:\[[^\]/?#@\[]*\]|(?:` + reUnres + `|` + rePct + `|` + reSub + `)*)`
	reAuth    = `(?:` + reUser + `@)?` + reHost + `(?::\d*)?`
	reAbempty = `(?:/` + reSegment + `)*`
	reHier    = `(?://` + reAuth + reAbempty + `|/(?:` + reSegNZ + reAbempty + `)?|` + reSegNZ + reAbempty + `|)`
	reRelPart = `(?://` + reAuth + reAbempty + `|/(?:` + reSegNZ + reAbempty + `)?|` + reSegNZNC + reAbempty + `|)`
	reTail    = `(?:\?` + reQuery + `)?(?:#` + reQuery + `)?`
)

var (
	uriRe      = regexp.MustCompile(`^` + reScheme + `:` + reHier + reTail + `$`)
	relativeRe = regexp.MustCompile(`^` + reRelPart + reTail + `$`)
	literalRe  = regexp.MustCompile(`\[([^\]]*)\]`)
	futureRe   = regexp.MustCompile(`^[vV][0-9A-Fa-f]+\.(?:` + reUnres + `|` + reSub + `|:)+$`)
)

// checkLiterals checks the IP literals of a URI, which the regular
// expressions leave open.
func checkLiterals(s string) string {
	for _, m := range literalRe.FindAllStringSubmatch(s, -1) {
		if !futureRe.MatchString(m[1]) && checkRFC4291(m[1], 1, false) != "" {
			return "bad IP literal [" + m[1] + "]"
		}
	}
	return ""
}

func TestRFC3986URI(t *testing.T) {
	checkValid(t, RFC3986URI(), func(s string) string {
		if !uriRe.MatchString(s) {
			return "not an RFC 3986 URI"
		}
		return checkLiterals(s)
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC3986URI().Draw(pt, "s")
		pt.Cover(strings.Contains(s, "://"), 30, "authority")
		pt.Cover(strings.Contains(s, "@"), 5, "userinfo or @")
		pt.Cover(strings.Contains(s, "["), 5, "IP literal")
		pt.Cover(strings.Contains(s, "[v"), 1, "IPvFuture")
		pt.Cover(strings.Contains(s, "%"), 10, "percent-encoded")
		pt.Cover(strings.Contains(s, "?"), 20, "query")
		pt.Cover(strings.Contains(s, "#"), 20, "fragment")
		pt.Cover(!strings.Contains(s, "/"), 5, "no slash")
		pt.Cover(strings.Contains(s, "[V"), 1, "IPvFuture with an upper-case V")
		pt.Cover(regexp.MustCompile(`:\d{6,}`).MatchString(s), 1, "port of six digits or more")
		pt.Cover(regexp.MustCompile(`%(?:[0-9][a-f]|[a-f][0-9A-F]|[A-F][a-f])`).MatchString(s), 2, "percent-encoding in mixed or lower case")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC3986URI(), always, uriTargets("http")...)
	shrinksTo(t, RFC3986URI(), func(s string) bool { return strings.HasPrefix(s, "http://") },
		"http://a", "http://", "http://0.0.0.0", "http://[::]", "http://[v0.a]")
}

// uriTargets lists the shrink targets the godoc of RFC3986URI names, for
// the given schemes.
func uriTargets(schemes ...string) []string {
	var ts []string
	for _, sc := range append(schemes, "a") {
		for _, h := range []string{"//a", "/", "a", "", "//", "//0.0.0.0", "//[::]", "//[v0.a]"} {
			ts = append(ts, sc+":"+h)
		}
	}
	return ts
}

func TestRFC3986Reference(t *testing.T) {
	checkValid(t, RFC3986Reference(), func(s string) string {
		if !uriRe.MatchString(s) && !relativeRe.MatchString(s) {
			return "not an RFC 3986 URI reference"
		}
		return checkLiterals(s)
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC3986Reference().Draw(pt, "s")
		pt.Cover(!uriRe.MatchString(s), 30, "relative")
		pt.Cover(strings.HasPrefix(s, "//"), 5, "network-path reference")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC3986Reference(), always, append(uriTargets("http"), "//a", "/", "a", "", "//", "//0.0.0.0", "//[::]", "//[v0.a]")...)
}

func TestURL(t *testing.T) {
	checkValid(t, URL(), func(s string) string {
		if !uriRe.MatchString(s) {
			return "not an RFC 3986 URI"
		}
		u, err := url.Parse(s)
		if err != nil {
			return err.Error()
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || !strings.HasPrefix(s, u.Scheme+"://"+u.Host) {
			return "url.Parse reads another scheme or host: " + u.Scheme + " " + u.Host
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := URL().Draw(pt, "s")
		u, _ := url.Parse(s)
		pt.Cover(u.Port() != "", 20, "port")
		pt.Cover(u.RawQuery != "", 20, "query")
		pt.Cover(strings.Contains(u.Host, "["), 5, "IPv6 host")
		pt.Cover(strings.Contains(s, "%"), 10, "percent-encoded")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, URL(), always, "http://a", "http://0.0.0.0", "http://[::]")
}

// TestGoURLDivergences pins what RFC3986URI's godoc says about
// net/url.Parse.
func TestGoURLDivergences(t *testing.T) {
	for _, s := range []string{"http://%41/", "http://a%7Eb/", "http://[v1.x]/", "http://[vA.a:b]/"} {
		if !uriRe.MatchString(s) || checkLiterals(s) != "" {
			t.Fatalf("%s is not a valid URI", s)
		}
		if _, err := url.Parse(s); err == nil {
			t.Errorf("url.Parse now accepts %s; update the godoc of RFC3986URI", s)
		}
	}
	for _, s := range []string{"http://%25/", "http://%C3%A9/", "http://a/%41?%41#%41", "http://a:/", "x:a:b"} {
		if _, err := url.Parse(s); err != nil {
			t.Errorf("url.Parse now rejects %s: %v; update the godoc of RFC3986URI", s, err)
		}
	}
}
