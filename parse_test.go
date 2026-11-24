package syntax_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

func TestHeader(t *testing.T) {
	for _, tt := range []struct {
		input, kind, scope, description string
		breaking                        bool
	}{
		{"build: pack archive", "build", "", "pack archive", false},
		{"Build(store)!: change key layout", "Build", "store", "change key layout", true},
		{"fix!: remove fallback", "fix", "", "remove fallback", true},
		{"ops(data store): rebuild", "ops", "data store", "rebuild", false},
		{"修复(缓存): 保留内容", "修复", "缓存", "保留内容", false},
		{"x-2(a/b):  retain spaces  \r\n", "x-2", "a/b", " retain spaces  ", false},
		{"docs: a: b! (c) 😀", "docs", "", "a: b! (c) 😀", false},
	} {
		t.Run(tt.input, func(t *testing.T) {
			c, err := syntax.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if c.Message() != tt.input || c.Type() != tt.kind || c.Scope() != tt.scope || c.Description() != tt.description || c.IsBreakingChange() != tt.breaking {
				t.Fatalf("unexpected fields: %#v", c)
			}
			for _, field := range []struct {
				span syntax.Span
				text string
			}{
				{c.HeaderSpan(), c.Header()}, {c.TypeSpan(), c.Type()},
				{c.ScopeSpan(), c.Scope()}, {c.DescriptionSpan(), c.Description()},
			} {
				if tt.input[field.span.Start:field.span.End] != field.text {
					t.Fatal("incorrect source span")
				}
			}
		})
	}
}

func TestHeaderErrors(t *testing.T) {
	for _, tt := range []struct {
		input  string
		code   syntax.ErrorCode
		offset int
	}{
		{"", syntax.InvalidType, 0},
		{" build: pack", syntax.InvalidType, 0},
		{"(store): pack", syntax.InvalidType, 0},
		{"---: pack", syntax.InvalidType, 3},
		{"build", syntax.InvalidSeparator, 5},
		{"build!!: pack", syntax.InvalidSeparator, 6},
		{"build(): pack", syntax.InvalidScope, 6},
		{"build( store): pack", syntax.InvalidScope, 6},
		{"build(store ): pack", syntax.InvalidScope, 11},
		{"build((store): pack", syntax.InvalidScope, 6},
		{"build(store", syntax.InvalidScope, 11},
		{"build:pack", syntax.InvalidSeparator, 6},
		{"build:\tpack", syntax.InvalidSeparator, 6},
		{"build:", syntax.InvalidSeparator, 6},
		{"build: ", syntax.InvalidDescription, 7},
		{"build: \t\u2003", syntax.InvalidDescription, 7},
		{"build: pack\r", syntax.InvalidCharacter, 11},
		{"build: pack\x00", syntax.InvalidCharacter, 11},
		{"build: pack\xff", syntax.InvalidEncoding, 11},
		{"bad header\n\xff", syntax.InvalidEncoding, 11},
	} {
		t.Run(tt.input, func(t *testing.T) {
			c, err := syntax.Parse(tt.input)
			var pe *syntax.ParseError
			if !errors.As(err, &pe) || pe.Code != tt.code || pe.Offset != tt.offset {
				t.Fatalf("error = %v, want code %d at %d", err, tt.code, tt.offset)
			}
			if c.Message() != "" || c.FooterCount() != 0 {
				t.Fatal("partial result on failure")
			}
			if pe.Error() == "" {
				t.Fatal("empty diagnostic")
			}
		})
	}
}

func TestZeroCommit(t *testing.T) {
	var c syntax.Commit
	if c.Message()+c.Header()+c.Type()+c.Scope()+c.Description()+c.Body()+c.Footer() != "" || c.FooterCount() != 0 || c.IsBreakingChange() {
		t.Fatal("nonempty zero value")
	}
}

func TestSections(t *testing.T) {
	type note struct {
		token, value string
		sep          syntax.Separator
	}
	for _, tt := range []struct {
		name, tail, body, footer string
		notes                    []note
		breaking                 bool
	}{
		{name: "no sections"},
		{name: "terminal empty lines", tail: "\n\n\n"},
		{name: "paragraphs", tail: "\n\none\n\ntwo\n", body: "one\n\ntwo"},
		{name: "extra separator", tail: "\n\n\ntext", body: "text"},
		{name: "body-like token", tail: "\n\ntext\nTrace: still body", body: "text\nTrace: still body"},
		{name: "hash", tail: "\n\nRefs #81", footer: "Refs #81", notes: []note{{"Refs", "81", syntax.Hash}}},
		{name: "colon", tail: "\n\nTrace: #81", footer: "Trace: #81", notes: []note{{"Trace", "#81", syntax.Colon}}},
		{name: "preserve spaces", tail: "\n\nTrace:  x  \n", footer: "Trace:  x  ", notes: []note{{"Trace", " x  ", syntax.Colon}}},
		{name: "body and footer", tail: "\n\ntext\n\nRefs #81\n", body: "text", footer: "Refs #81", notes: []note{{"Refs", "81", syntax.Hash}}},
		{name: "continuation", tail: "\n\nTrace: first\nsecond\n\nthird", footer: "Trace: first\nsecond\n\nthird", notes: []note{{"Trace", "first\nsecond\n\nthird", syntax.Colon}}},
		{name: "duplicates", tail: "\n\nTrace: a\nTrace: b", footer: "Trace: a\nTrace: b", notes: []note{{"Trace", "a", syntax.Colon}, {"Trace", "b", syntax.Colon}}},
		{name: "blank between entries", tail: "\n\nTrace: a\n\nRefs #81", footer: "Trace: a\n\nRefs #81", notes: []note{{"Trace", "a\n", syntax.Colon}, {"Refs", "81", syntax.Hash}}},
		{name: "indented continuation", tail: "\n\nTrace: a\n Refs #81", footer: "Trace: a\n Refs #81", notes: []note{{"Trace", "a\n Refs #81", syntax.Colon}}},
		{name: "breaking spaced", tail: "\n\nBREAKING CHANGE: drop alias", footer: "BREAKING CHANGE: drop alias", notes: []note{{"BREAKING CHANGE", "drop alias", syntax.Colon}}, breaking: true},
		{name: "breaking hyphenated", tail: "\n\nBREAKING-CHANGE: drop alias", footer: "BREAKING-CHANGE: drop alias", notes: []note{{"BREAKING-CHANGE", "drop alias", syntax.Colon}}, breaking: true},
		{name: "mixed-case ordinary", tail: "\n\nBreaking-Change: drop alias", footer: "Breaking-Change: drop alias", notes: []note{{"Breaking-Change", "drop alias", syntax.Colon}}},
		{name: "spaced wrong case", tail: "\n\nbreaking change: text", body: "breaking change: text"},
		{name: "breaking hash", tail: "\n\nBREAKING-CHANGE #81", body: "BREAKING-CHANGE #81"},
		{name: "unicode token", tail: "\n\n追跡: 値", footer: "追跡: 値", notes: []note{{"追跡", "値", syntax.Colon}}},
		{name: "hyphenated token", tail: "\n\nChecked-by: Kim", footer: "Checked-by: Kim", notes: []note{{"Checked-by", "Kim", syntax.Colon}}},
		{name: "empty candidate", tail: "\n\nTrace: ", body: "Trace: "},
		{name: "value on next line", tail: "\n\nTrace: \n  detail", footer: "Trace: \n  detail", notes: []note{{"Trace", "\n  detail", syntax.Colon}}},
		{name: "punctuated body", tail: "\n\nA/B: text", body: "A/B: text"},
		{name: "fullwidth colon", tail: "\n\nRefs\uff1a x", body: "Refs\uff1a x"},
		{name: "nonempty whitespace body", tail: "\n\n \n", body: " "},
		{name: "multiple breaking footers", tail: "\n\nBREAKING CHANGE: x\nBREAKING-CHANGE: y", footer: "BREAKING CHANGE: x\nBREAKING-CHANGE: y", notes: []note{{"BREAKING CHANGE", "x", syntax.Colon}, {"BREAKING-CHANGE", "y", syntax.Colon}}, breaking: true},
	} {
		for _, ending := range []string{"\n", "\r\n"} {
			t.Run(tt.name+ending, func(t *testing.T) {
				replace := func(s string) string { return strings.ReplaceAll(s, "\n", ending) }
				input := "build: pack archive" + replace(tt.tail)
				c, err := syntax.Parse(input)
				if err != nil {
					t.Fatal(err)
				}
				if c.Message() != input || c.Header() != "build: pack archive" || c.Body() != replace(tt.body) || c.Footer() != replace(tt.footer) || c.FooterCount() != len(tt.notes) || c.IsBreakingChange() != tt.breaking {
					t.Fatalf("body=%q footer=%q entries=%d breaking=%v", c.Body(), c.Footer(), c.FooterCount(), c.IsBreakingChange())
				}
				for i, want := range tt.notes {
					f := c.FooterAt(i)
					if f.Token() != want.token || f.Value() != replace(want.value) || f.Separator() != want.sep {
						t.Fatalf("footer %d = %#v", i, f)
					}
					if input[f.TokenSpan().Start:f.TokenSpan().End] != want.token || input[f.ValueSpan().Start:f.ValueSpan().End] != replace(want.value) {
						t.Fatalf("footer %d spans mismatch", i)
					}
				}
				for _, field := range []struct {
					span syntax.Span
					text string
				}{{c.BodySpan(), c.Body()}, {c.FooterSpan(), c.Footer()}} {
					if input[field.span.Start:field.span.End] != field.text {
						t.Fatal("incorrect section span")
					}
				}
			})
		}
	}
}

func TestMissingSectionSeparator(t *testing.T) {
	for _, tail := range []string{"\nbody", "\r\nbody", "\n \nbody", "\nRefs #8"} {
		input := "fix: rebuild" + tail
		c, err := syntax.Parse(input)
		var pe *syntax.ParseError
		if !errors.As(err, &pe) || pe.Code != syntax.InvalidSectionSeparator || pe.Offset != 12+len(tail)-len(strings.TrimLeft(tail, "\r\n")) || c.Message() != "" {
			t.Fatalf("input %q: %v", input, err)
		}
	}
}

func TestMixedLineEndingsAndBang(t *testing.T) {
	input := "fix!: rebuild\r\n\nbody\r\n\r\nTrace: a\nRefs #8\r\n\n"
	c, err := syntax.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsBreakingChange() || c.Body() != "body" || c.FooterCount() != 2 || c.Footer() != "Trace: a\nRefs #8" {
		t.Fatalf("unexpected sections: %#v", c)
	}
}

func TestEmptyValueLookahead(t *testing.T) {
	for _, tt := range []struct {
		tail, body, footer string
		count              int
	}{
		{"Trace: \nNext: text", "Trace: \nNext: text", "", 0},
		{"Trace: \n\n continuation\nNext: text", "", "Trace: \n\n continuation\nNext: text", 2},
		{"BREAKING CHANGE: \n explanation", "", "BREAKING CHANGE: \n explanation", 1},
	} {
		c, err := syntax.Parse("ops: rebuild\n\n" + tt.tail)
		if err != nil {
			t.Fatal(err)
		}
		if c.Body() != tt.body || c.Footer() != tt.footer || c.FooterCount() != tt.count {
			t.Fatalf("tail %q: body=%q footer=%q count=%d", tt.tail, c.Body(), c.Footer(), c.FooterCount())
		}
	}
}

func TestInvalidFooterValues(t *testing.T) {
	for _, tt := range []struct {
		tail   string
		offset int
	}{
		{"Trace: a\nEmpty: ", 16},
		{"Trace: text\nEmpty: \nNext: value", 19},
		{"Trace: a\nEmpty #", 16},
		{"Trace: a\nEmpty: \t\u2003", 16},
		{"BREAKING CHANGE: ", 17},
		{"BREAKING-CHANGE: ", 17},
		{"BREAKING CHANGE: \nRefs #9", 17},
		{"text\n\nBREAKING-CHANGE: ", 23},
	} {
		for _, ending := range []string{"\n", "\r\n"} {
			tail := strings.ReplaceAll(tt.tail, "\n", ending)
			input := "ops: rebuild" + ending + ending + tail
			c, err := syntax.Parse(input)
			var pe *syntax.ParseError
			prefix := "ops: rebuild" + ending + ending
			wantOffset := len(prefix) + tt.offset + strings.Count(tt.tail[:tt.offset], "\n")*(len(ending)-1)
			if !errors.As(err, &pe) || pe.Code != syntax.InvalidFooterValue || pe.Offset != wantOffset || c.Message() != "" {
				t.Fatalf("input %q: error=%v, want byte %d", input, err, wantOffset)
			}
		}
	}
}

func TestConcurrentParsingAndAccess(t *testing.T) {
	input := "ops(store)!: rebuild\n\nbody\n\nRefs #91"
	shared, err := syntax.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		t.Run("reader", func(t *testing.T) {
			t.Parallel()
			for j := 0; j < 100; j++ {
				c, err := syntax.Parse(input)
				if err != nil {
					t.Fatal(err)
				}
				if c.Message() != shared.Message() || c.FooterAt(0) != shared.FooterAt(0) || c.Body() != shared.Body() {
					t.Fatal("inconsistent concurrent result")
				}
			}
		})
	}
}
