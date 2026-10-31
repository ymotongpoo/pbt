// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Command site renders the user guide under docs/guide into the static
// pages served at https://go.ymotongpoo.com/pbt/guide/. It is a module of
// its own, so that the root module stays free of dependencies; run it from
// this directory:
//
//	cd site && go run . -version v0.2.0
//
// The English chapters are the *.md files of docs/guide, the Japanese ones
// those of docs/guide/ja; both are ordered by file name. A chapter includes
// a code listing with a line of the form
//
//	<!-- snippet: FILE#NAME -->
//
// which is replaced by the region of docs/guide/snippets/FILE between the
// comments "// snippet: NAME" and "// snippet: end".
package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

//go:embed template.html
var templateSrc string

const canonicalBase = "https://go.ymotongpoo.com/pbt/guide/"

type language struct {
	code, dir, guideTitle, otherLabel, otherDir string
}

var languages = []language{
	{code: "en", dir: "", guideTitle: "pbt user guide", otherLabel: "日本語", otherDir: "ja/"},
	{code: "ja", dir: "ja", guideTitle: "pbt 利用ガイド", otherLabel: "English", otherDir: "../"},
}

type chapter struct {
	file  string
	out   string // file name written, index.html or NN-slug.html
	href  string // link to the page, "./" or NN-slug: the site serves clean URLs
	title string
	body  template.HTML
}

type navItem struct {
	Href    string
	Title   string
	Current bool
}

type page struct {
	Lang       string
	Title      string
	GuideTitle string
	Version    string
	Canonical  string
	OtherLang  string
	OtherLabel string
	Content    template.HTML
	Nav        []navItem
	Prev, Next *navItem
}

func main() {
	src := flag.String("src", "../docs/guide", "directory of the guide's Markdown sources")
	out := flag.String("out", "public", "directory the pages are written to")
	version := flag.String("version", "", "pbt version the guide describes, shown in the footer")
	flag.Parse()
	if err := run(*src, *out, *version); err != nil {
		fmt.Fprintln(os.Stderr, "site:", err)
		os.Exit(1)
	}
}

func run(src, out, version string) error {
	tmpl, err := template.New("page").Parse(templateSrc)
	if err != nil {
		return err
	}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft)),
			highlighting.NewHighlighting(highlighting.WithStyle("github")),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	for _, lang := range languages {
		chapters, err := loadChapters(md, filepath.Join(src, lang.dir), filepath.Join(src, "snippets"))
		if err != nil {
			return fmt.Errorf("%s: %w", lang.code, err)
		}
		if len(chapters) == 0 {
			continue
		}
		for i, ch := range chapters {
			nav := make([]navItem, len(chapters))
			for j, other := range chapters {
				nav[j] = navItem{Href: other.href, Title: other.title, Current: i == j}
			}
			leaf := ch.href
			if i == 0 {
				leaf = ""
			}
			p := page{
				Lang:       lang.code,
				Title:      ch.title,
				GuideTitle: lang.guideTitle,
				Version:    version,
				Canonical:  canonicalBase + path(lang.dir, leaf),
				OtherLang:  lang.otherDir + leaf,
				OtherLabel: lang.otherLabel,
				Content:    ch.body,
				Nav:        nav,
			}
			if i > 0 {
				p.Prev = &nav[i-1]
			}
			if i+1 < len(chapters) {
				p.Next = &nav[i+1]
			}
			var b bytes.Buffer
			if err := tmpl.Execute(&b, p); err != nil {
				return err
			}
			dst := filepath.Join(out, lang.dir, ch.out)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dst, b.Bytes(), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// path joins URL path elements, which never use the OS separator.
func path(dir, file string) string {
	if dir == "" {
		return file
	}
	return dir + "/" + file
}

// linkOf returns the clean URL of a chapter file relative to its language
// directory: the index is "./", and NN-slug.html is served as NN-slug.
func linkOf(out string) string {
	if out == "index.html" {
		return "./"
	}
	return strings.TrimSuffix(out, ".html")
}

var snippetRef = regexp.MustCompile(`(?m)^<!-- snippet: ([^#\s]+)#(\S+) -->[ \t]*$`)

// loadChapters reads the chapters of one language in file-name order. The
// first chapter becomes index.html; the others keep their name with the
// .html extension, and links leave the extension out because the site
// serves clean URLs.
func loadChapters(md goldmark.Markdown, dir, snippets string) ([]chapter, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var chapters []chapter
	for i, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		text, err := expandSnippets(string(data), snippets)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		title := ""
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
		if title == "" {
			return nil, fmt.Errorf("%s: no level-1 heading", name)
		}
		var b bytes.Buffer
		if err := md.Convert([]byte(text), &b); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out := strings.TrimSuffix(name, ".md") + ".html"
		if i == 0 {
			out = "index.html"
		}
		chapters = append(chapters, chapter{file: name, out: out, href: linkOf(out), title: title, body: template.HTML(b.String())})
	}
	return chapters, nil
}

// expandSnippets replaces every snippet reference with a Go code block.
func expandSnippets(text, dir string) (string, error) {
	var firstErr error
	text = snippetRef.ReplaceAllStringFunc(text, func(ref string) string {
		m := snippetRef.FindStringSubmatch(ref)
		code, err := region(filepath.Join(dir, m[1]), m[2])
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return "```go\n" + code + "\n```"
	})
	return text, firstErr
}

// region returns the lines of path between "// snippet: name" and the next
// "// snippet: end", with the common indentation removed.
func region(path, name string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, line := range lines {
		switch t := strings.TrimSpace(line); {
		case start < 0 && t == "// snippet: "+name:
			start = i + 1
		case start >= 0 && t == "// snippet: end":
			return dedent(lines[start:i]), nil
		}
	}
	return "", fmt.Errorf("%s: no snippet %q", path, name)
}

func dedent(lines []string) string {
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, "\t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, l[indent:])
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}
