package syntax_test

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/conventionalcommit/syntax"
)

func FuzzParse(f *testing.F) {
	for _, fixture := range readRealCommits(f) {
		f.Add(fixture.Message)
	}
	for _, seed := range []string{
		"", "build(cache)!: repack", "docs: retain 😀\r\n",
		"fix: reload\n\nbody\n\nTrace: a\nRefs #9",
		"fix: reload\n\nTrace: \n\nvalue\nEmpty: ",
		"fix: reload\n\nBREAKING CHANGE: remove key",
		"fix: reload\n\nBREAKING-CHANGE #9",
		"fix: \xff", "fix: x\x00", "fix: x\r",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		c, err := syntax.Parse(input)
		again, againErr := syntax.Parse(input)
		if err != nil {
			var pe, other *syntax.ParseError
			if !errors.As(err, &pe) || !errors.As(againErr, &other) || *pe != *other {
				t.Fatal("inconsistent errors")
			}
			if pe.Offset < 0 || pe.Offset > len(input) {
				t.Fatal("error outside input")
			}
			if !reflect.DeepEqual(c, syntax.Commit{}) {
				t.Fatal("partial result on failure")
			}
			return
		}
		if againErr != nil || !reflect.DeepEqual(c, again) || c.Message() != input || !utf8.ValidString(input) {
			t.Fatal("inconsistent result")
		}
		check := func(span syntax.Span, text string) {
			t.Helper()
			if span.Start < 0 || span.End < span.Start || span.End > len(input) {
				t.Fatal("span outside input")
			}
			if text != input[span.Start:span.End] || !utf8.ValidString(text) {
				t.Fatal("invalid field")
			}
		}
		check(c.HeaderSpan(), c.Header())
		check(c.TypeSpan(), c.Type())
		check(c.ScopeSpan(), c.Scope())
		check(c.DescriptionSpan(), c.Description())
		check(c.BodySpan(), c.Body())
		check(c.FooterSpan(), c.Footer())
		if c.HeaderSpan().Start != 0 || c.Type() == "" || c.Description() == "" {
			t.Fatal("missing header")
		}
		previous := c.FooterSpan().Start
		for i := 0; i < c.FooterCount(); i++ {
			entry := c.FooterAt(i)
			check(entry.TokenSpan(), entry.Token())
			check(entry.ValueSpan(), entry.Value())
			if entry.TokenSpan().Start < previous || entry.TokenSpan().End > entry.ValueSpan().Start || entry.ValueSpan().End > c.FooterSpan().End {
				t.Fatal("unordered footer spans")
			}
			if entry.Separator() != syntax.Colon && entry.Separator() != syntax.Hash {
				t.Fatal("invalid separator")
			}
			previous = entry.ValueSpan().End
		}
	})
}

func FuzzInspect(f *testing.F) {
	for _, fixture := range readRealCommits(f) {
		f.Add(fixture.Message)
	}
	for _, input := range []string{"", "feat: expose records", "\tfeat:change\nBody\n\nBREAKING CHANGE: \nEmpty: ", "feat: x\nRefs #9", "feat: x\xff", "feat: x\n\nRefs #9\nNext: "} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got := syntax.Inspect(input)
		if !reflect.DeepEqual(got, syntax.Inspect(input)) || got.Commit.Message() != input {
			t.Fatal("inspection is not deterministic")
		}
		strict, err := syntax.Parse(input)
		if err == nil {
			if !got.Structured || len(got.Diagnostics) != 0 || !reflect.DeepEqual(strict, got.Commit) {
				t.Fatal("valid input disagrees with strict parsing")
			}
		} else {
			if len(got.Diagnostics) == 0 || got.Diagnostics[0] != *err.(*syntax.ParseError) {
				t.Fatal("first diagnostic disagrees with strict parsing")
			}
		}
		previous := 0
		for _, issue := range got.Diagnostics {
			if issue.Offset < previous || issue.Offset > len(input) {
				t.Fatal("diagnostic order or bounds invalid")
			}
			previous = issue.Offset
		}
		c := got.Commit
		check := func(span syntax.Span) {
			t.Helper()
			if span.Start < 0 || span.End < span.Start || span.End > len(input) {
				t.Fatal("span outside input")
			}
		}
		for _, span := range []syntax.Span{c.HeaderSpan(), c.TypeSpan(), c.ScopeSpan(), c.DescriptionSpan(), c.BodySpan(), c.FooterSpan(), c.HeaderBangSpan(), c.BodyGap().Span, c.FooterGap().Span} {
			check(span)
		}
		for i := 0; i < c.FooterCount(); i++ {
			entry := c.FooterAt(i)
			check(entry.TokenSpan())
			check(entry.ValueSpan())
		}
		if !got.Structured && (c.Type()+c.Scope()+c.Description()+c.Body()+c.Footer() != "" || c.FooterCount() != 0) {
			t.Fatal("ambiguous structure exposed")
		}
		it := syntax.Lines(input)
		end, count := 0, 0
		for line, ok := it.Next(); ok; line, ok = it.Next() {
			check(line.Content)
			check(line.Ending)
			if line.Content.Start != end || line.Content.End != line.Ending.Start {
				t.Fatal("line ranges are not contiguous")
			}
			end = line.Ending.End
			count++
			if count > len(input)+1 {
				t.Fatal("too many lines")
			}
		}
		if end != len(input) {
			t.Fatal("line iteration lost bytes")
		}
	})
}

// This fuzzer constructs valid messages with known fields. The arbitrary-byte
// fuzzers separately cover malformed input without modifying its bytes.
func FuzzValidMessages(f *testing.F) {
	for i, text := range corpusDescriptions {
		f.Add(text, uint8(i*41), uint16(i))
	}
	f.Fuzz(func(t *testing.T, text string, flags uint8, ordinal uint16) {
		if len(text) > 8192 {
			text = text[:8192]
		}
		text = strings.Map(func(r rune) rune {
			if r == 0 || r == '\r' || r == '\n' {
				return ' '
			}
			return r
		}, strings.ToValidUTF8(text, "\ufffd"))
		kind := []string{"feat", "fix", "chore", "修复"}[flags&3]
		scope := []string{"", "api", "store/v2"}[(flags>>2)%3]
		bang := flags&16 != 0
		nl := "\n"
		if flags&32 != 0 {
			nl = "\r\n"
		}
		header := kind
		if scope != "" {
			header += "(" + scope + ")"
		}
		if bang {
			header += "!"
		}
		description := "entry " + text
		header += ": " + description
		input := header
		body, footer := "", ""
		var entries []expectedNote
		if flags&64 != 0 {
			body = "  body " + text + nl + "  next " + text
			input += nl + nl + body
		}
		if flags&128 != 0 {
			value := strconv.Itoa(int(ordinal))
			footer = "Refs #" + value + nl + "Trace: note " + text
			entries = []expectedNote{{"Refs", value, syntax.Hash}, {"Trace", "note " + text, syntax.Colon}}
			input += nl + nl + footer
		}
		checkMessage(t, input, expectedCommit{header: header, kind: kind, scope: scope, description: description, body: body, footer: footer, entries: entries, bang: bang})
	})
}
