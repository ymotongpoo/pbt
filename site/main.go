// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Command site renders the user guide under docs/guide into the static
// pages served at https://go.ymotongpoo.com/pbt/guide/. It is a module of
// its own, so that the root module stays free of dependencies; run it from
// this directory:
//
//	cd site && go run . -version v0.3.0 -analytics G-XXXXXXXXXX
//
// The -analytics flag takes a Google Analytics measurement ID; when it is
// given, every page loads gtag.js for it. Leave it out for a local preview.
// The output, site/public, is copied into public/guide/pbt of the
// repository github.com/ymotongpoo/go.ymotongpoo.com, which serves the
// domain; AGENTS.md describes the steps.
//
// The English chapters are the *.md files of docs/guide, the Japanese ones
// those of docs/guide/ja; both are ordered by file name. A chapter includes
// a code listing with a line of the form
//
//	<!-- snippet: FILE#NAME -->
//
// which is replaced by the region of docs/guide/snippets/FILE between the
// comments "// snippet: NAME" and "// snippet: end". The line after the
// title of a chapter is
//
//	<!-- summary: one sentence -->
//
// which describes the chapter in the llms.txt index.
//
// Besides the HTML pages, the command writes what AI agents read, following
// the llms.txt proposal at https://llmstxt.org/: a Markdown version of every
// page next to it, with .md in place of .html, which the page announces
// with a link of rel="alternate" and type="text/markdown"; llms-full.txt per
// language, the whole guide in one file; and llms.txt at the root of the
// output, the index of the documentation with a line per page.
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
	file    string
	out     string // file name written, index.html or NN-slug.html
	md      string // the Markdown version, index.md or NN-slug.md
	href    string // link to the page, "./" or NN-slug: the site serves clean URLs
	title   string
	summary string // the one-line description from the summary comment
	text    string // the Markdown source with the snippets expanded
	body    template.HTML
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
	Analytics  string // Google Analytics measurement ID, empty for none
	Canonical  string
	Markdown   string // relative link to the Markdown version of the page
	LlmsTxt    string // absolute path of the llms.txt index
	OtherLang  string
	OtherLabel string
	Content    template.HTML
	Nav        []navItem
	Prev, Next *navItem
}

// llmsTxtPath is where the site serves the llms.txt index; the pages branch
// maps it, and /llms.txt, onto the file this command writes at the root of
// its output.
const llmsTxtPath = "/pbt/llms.txt"

func main() {
	src := flag.String("src", "../docs/guide", "directory of the guide's Markdown sources")
	out := flag.String("out", "public", "directory the pages are written to")
	version := flag.String("version", "", "pbt version the guide describes, shown in the footer")
	analytics := flag.String("analytics", "", "Google Analytics measurement ID to load on every page")
	flag.Parse()
	if err := run(*src, *out, *version, *analytics); err != nil {
		fmt.Fprintln(os.Stderr, "site:", err)
		os.Exit(1)
	}
}

func run(src, out, version, analytics string) error {
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
	byLang := map[string][]chapter{}
	for _, lang := range languages {
		chapters, err := loadChapters(md, filepath.Join(src, lang.dir), filepath.Join(src, "snippets"))
		if err != nil {
			return fmt.Errorf("%s: %w", lang.code, err)
		}
		if len(chapters) == 0 {
			continue
		}
		byLang[lang.code] = chapters
		if err := writeFile(filepath.Join(out, lang.dir, "llms-full.txt"), fullGuide(lang, chapters, version)); err != nil {
			return err
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
				Analytics:  analytics,
				Canonical:  canonicalBase + path(lang.dir, leaf),
				Markdown:   ch.md,
				LlmsTxt:    llmsTxtPath,
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
			if err := writeFile(filepath.Join(out, lang.dir, ch.out), b.String()); err != nil {
				return err
			}
			if err := writeFile(filepath.Join(out, lang.dir, ch.md), ch.text); err != nil {
				return err
			}
		}
	}
	return writeFile(filepath.Join(out, "llms.txt"), llmsTxt(byLang, version))
}

func writeFile(dst, content string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(content), 0o644)
}

// llmsTxt is the index of the documentation in the format of
// https://llmstxt.org/: the name, a summary, the sections of links with a
// description each, and the secondary links under Optional.
func llmsTxt(byLang map[string][]chapter, version string) string {
	var b strings.Builder
	b.WriteString("# pbt\n\n")
	b.WriteString("> pbt is a property-based testing library for Go with the feature set of Erlang's PropEr: generators whose distribution you control, statistics that check it, stateful and parallel command testing, state machines, targeted search and existential properties. Module path go.ymotongpoo.com/pbt; Apache License 2.0; Go 1.25 or later.\n\n")
	fmt.Fprintf(&b, "This file indexes the documentation of pbt %s. Every page of the user guide has a Markdown version at its URL with .md appended, and llms-full.txt holds the whole guide in one file. The package documentation on pkg.go.dev is the reference for every function and option; the repository holds the design notes, the migration guide for rapid users, the changelog and the examples. Stateful tests are written as commands that close over a model and a system kept in the test's variables, with no type parameters; generators are typed, and the type is inferred in nearly every call.\n\n", version)
	b.WriteString("## User guide\n\n")
	for _, ch := range byLang["en"] {
		fmt.Fprintf(&b, "- [%s](%s%s): %s\n", ch.title, canonicalBase, ch.md, ch.summary)
	}
	b.WriteString("\n## Reference\n\n")
	b.WriteString("- [Package documentation](https://pkg.go.dev/go.ymotongpoo.com/pbt): every function and option of package pbt, with runnable examples\n")
	b.WriteString("- [Package fsm](https://pkg.go.dev/go.ymotongpoo.com/pbt/fsm): state machines with named states and weighted transitions\n")
	b.WriteString("- [README](https://raw.githubusercontent.com/ymotongpoo/pbt/main/README.md): what pbt is, a first property and a stateful property\n")
	b.WriteString("- [CHANGELOG](https://raw.githubusercontent.com/ymotongpoo/pbt/main/CHANGELOG.md): the changes of each release and how to upgrade\n")
	b.WriteString("- [MIGRATION](https://raw.githubusercontent.com/ymotongpoo/pbt/main/MIGRATION.md): rapid's API mapped onto pbt, table by table\n")
	b.WriteString("- [DESIGN](https://raw.githubusercontent.com/ymotongpoo/pbt/main/DESIGN.md): the design decisions, the API listing and the comparison with PropEr, QuickCheck, Hypothesis and rapid\n")
	b.WriteString("- [Examples](https://github.com/ymotongpoo/pbt/tree/main/examples): the PropEr Testing book ported to Go, and practical stateful tests of an HTTP API, a job queue, a rate limiter, a session store and a ledger\n")
	b.WriteString("- [AGENTS.md](https://raw.githubusercontent.com/ymotongpoo/pbt/main/AGENTS.md): instructions for coding agents that work on the repository itself\n")
	if ja := byLang["ja"]; len(ja) > 0 {
		b.WriteString("\n## 利用ガイド（日本語）\n\n")
		for _, ch := range ja {
			fmt.Fprintf(&b, "- [%s](%sja/%s): %s\n", ch.title, canonicalBase, ch.md, ch.summary)
		}
	}
	b.WriteString("\n## Optional\n\n")
	fmt.Fprintf(&b, "- [The whole English guide in one file](%sllms-full.txt)\n", canonicalBase)
	if len(byLang["ja"]) > 0 {
		fmt.Fprintf(&b, "- [The whole Japanese guide in one file](%sja/llms-full.txt)\n", canonicalBase)
	}
	fmt.Fprintf(&b, "- [DESIGN.ja.md](https://raw.githubusercontent.com/ymotongpoo/pbt/main/DESIGN.ja.md): the design notes in Japanese\n")
	return b.String()
}

// fullGuide is llms-full.txt for one language: every chapter, in order,
// in one Markdown file.
func fullGuide(lang language, chapters []chapter, version string) string {
	var b strings.Builder
	if lang.code == "ja" {
		fmt.Fprintf(&b, "# pbt 利用ガイド（%s）\n\n> Goのプロパティベーステストライブラリpbtの利用ガイドを1つのファイルにまとめたものです。文書の索引は %sllms.txt にあります。\n", version, canonicalBase)
	} else {
		fmt.Fprintf(&b, "# pbt user guide (%s)\n\n> The user guide of pbt, a property-based testing library for Go, in one file. The index of the documentation is %sllms.txt.\n", version, canonicalBase)
	}
	for _, ch := range chapters {
		b.WriteString("\n---\n\n")
		b.WriteString(strings.TrimRight(ch.text, "\n"))
		b.WriteString("\n")
	}
	return b.String()
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

var (
	snippetRef = regexp.MustCompile(`(?m)^<!-- snippet: ([^#\s]+)#(\S+) -->[ \t]*$`)
	summaryRef = regexp.MustCompile(`(?m)^<!-- summary: (.+?) -->[ \t]*\n`)
)

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
		m := summaryRef.FindStringSubmatch(text)
		if m == nil {
			return nil, fmt.Errorf("%s: no summary comment after the title", name)
		}
		summary := strings.TrimSpace(m[1])
		text = strings.Replace(text, m[0], "", 1)
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
		chapters = append(chapters, chapter{
			file: name, out: out, md: strings.TrimSuffix(out, ".html") + ".md", href: linkOf(out),
			title: title, summary: summary, text: text, body: template.HTML(b.String()),
		})
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
