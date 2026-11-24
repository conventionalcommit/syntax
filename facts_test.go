package syntax_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

func TestBreakingFacts(t *testing.T) {
	for _, tt := range []struct {
		input        string
		bang, footer bool
	}{
		{"ops: retain ! in text", false, false},
		{"ops!: replace key", true, false},
		{"ops: replace key\n\nBREAKING CHANGE: remove alias", false, true},
		{"ops(store)!: replace key\n\nBREAKING-CHANGE: remove alias", true, true},
	} {
		c, err := syntax.Parse(tt.input)
		if err != nil {
			t.Fatal(err)
		}
		if c.HasHeaderBang() != tt.bang || c.HasBreakingFooter() != tt.footer || c.IsBreakingChange() != (tt.bang || tt.footer) {
			t.Fatalf("incorrect markers for %q", tt.input)
		}
		span := c.HeaderBangSpan()
		if tt.bang {
			if tt.input[span.Start:span.End] != "!" {
				t.Fatal("incorrect bang span")
			}
		} else if span != (syntax.Span{}) {
			t.Fatal("nonzero absent bang")
		}
	}
}

func TestSectionGaps(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		input := "ops: replace key" + nl + nl + nl + "body" + nl + nl + "Refs #3"
		c, err := syntax.Parse(input)
		if err != nil {
			t.Fatal(err)
		}
		body, footer := c.BodyGap(), c.FooterGap()
		if body.BlankLines != 2 || footer.BlankLines != 1 || input[body.Span.Start:body.Span.End] != strings.Repeat(nl, 3) || input[footer.Span.Start:footer.Span.End] != nl+nl {
			t.Fatalf("incorrect gaps: %#v %#v", body, footer)
		}
	}
	c, _ := syntax.Parse("ops: replace key\r\n\nRefs #3")
	if !c.BodyGap().IsZero() || c.FooterGap().BlankLines != 1 {
		t.Fatal("incorrect footer-only gap")
	}
	var zero syntax.Commit
	if zero.HasHeaderBang() || zero.HasBreakingFooter() || !zero.HeaderBangSpan().IsZero() || !zero.BodyGap().IsZero() || !zero.FooterGap().IsZero() {
		t.Fatal("nonzero absent facts")
	}
}

func TestLines(t *testing.T) {
	for _, tt := range []struct {
		input            string
		content, endings []string
	}{
		{"", []string{""}, []string{""}},
		{"a", []string{"a"}, []string{""}},
		{"a\n", []string{"a", ""}, []string{"\n", ""}},
		{"a\r\n\nb\n", []string{"a", "", "b", ""}, []string{"\r\n", "\n", "\n", ""}},
		{"\r", []string{"\r"}, []string{""}},
		{"\xff\n", []string{"\xff", ""}, []string{"\n", ""}},
	} {
		it := syntax.Lines(tt.input)
		var content, endings []string
		for line, ok := it.Next(); ok; line, ok = it.Next() {
			content = append(content, tt.input[line.Content.Start:line.Content.End])
			endings = append(endings, tt.input[line.Ending.Start:line.Ending.End])
			if line.Content.Len()+line.Ending.Len() != line.Ending.End-line.Content.Start {
				t.Fatal("noncontiguous line")
			}
		}
		if !reflect.DeepEqual(content, tt.content) || !reflect.DeepEqual(endings, tt.endings) {
			t.Fatalf("input=%q content=%q endings=%q", tt.input, content, endings)
		}
		if line, ok := it.Next(); ok || line != (syntax.Line{}) {
			t.Fatal("iterator did not remain exhausted")
		}
	}
	c, _ := syntax.Parse("ops: replace key\n\nbody\r\nnext")
	it := c.Lines(c.BodySpan())
	line, ok := it.Next()
	if !ok || line.Content.Start != c.BodySpan().Start || c.Message()[line.Content.Start:line.Content.End] != "body" || line.Ending.Len() != 2 {
		t.Fatal("scoped offsets are not absolute")
	}
	copy := it
	a, _ := it.Next()
	b, _ := copy.Next()
	if a != b {
		t.Fatal("iterator copies diverged")
	}
	emptyIt := c.Lines(syntax.Span{})
	emptyLine, ok := emptyIt.Next()
	if !ok || emptyLine != (syntax.Line{}) {
		t.Fatalf("empty span yielded %#v, %v", emptyLine, ok)
	}
	if _, ok := emptyIt.Next(); ok {
		t.Fatal("empty span iterator yielded more than one line")
	}
	for _, bad := range []syntax.Span{{-1, 1}, {0, len(c.Message()) + 1}, {5, 2}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Commit.Lines(%#v) did not panic", bad)
				}
			}()
			c.Lines(bad)
		}()
	}
}

var factsTotal int

func TestFactsAllocationBudget(t *testing.T) {
	c, _ := syntax.Parse("ops!: replace key\n\nbody\n\nRefs #3")
	got := testing.AllocsPerRun(100, func() {
		it := c.Lines(c.BodySpan())
		for line, ok := it.Next(); ok; line, ok = it.Next() {
			factsTotal += line.Content.Len()
		}
		factsTotal += c.BodyGap().BlankLines + c.FooterGap().BlankLines
		if c.HasHeaderBang() {
			factsTotal++
		}
		for line := range c.BodyLines() {
			factsTotal += line.Content.Len()
		}
		for line := range c.FooterLines() {
			factsTotal += line.Content.Len()
		}
		for f := range c.Footers() {
			factsTotal += f.TokenSpan().Len() + f.ValueSpan().Len()
		}
		factsTotal += c.Position(len(c.Message())).Line
	})
	if got != 0 {
		t.Fatalf("facts allocated %g times", got)
	}
}

func TestSpanAndCommitHelpers(t *testing.T) {
	if !(syntax.Span{}).IsZero() || !(syntax.Span{}).IsEmpty() {
		t.Fatal("zero span checks failed")
	}
	presentEmpty := syntax.Span{Start: 5, End: 5}
	if presentEmpty.IsZero() || !presentEmpty.IsEmpty() || presentEmpty.Contains(5) {
		t.Fatal("present-empty span checks failed")
	}
	nonEmpty := syntax.Span{Start: 2, End: 5}
	if nonEmpty.IsZero() || nonEmpty.IsEmpty() || nonEmpty.Contains(-1) || nonEmpty.Contains(1) || !nonEmpty.Contains(2) || !nonEmpty.Contains(4) || nonEmpty.Contains(5) || (syntax.Span{}).Contains(0) {
		t.Fatal("nonempty span checks failed")
	}

	var zeroFooter syntax.Footer
	if !zeroFooter.Span().IsZero() || !zeroFooter.SeparatorSpan().IsZero() || zeroFooter.IsBreaking() {
		t.Fatal("zero footer checks failed")
	}

	input := "feat(api)!: add endpoint\n\nBody one\nBody two\n\nRefs #10\nBREAKING CHANGE: drop v1\r\n\n"
	c, err := syntax.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		fn()
	}
	for _, bad := range []syntax.Span{{-1, 1}, {0, len(input) + 1}, {5, 2}} {
		mustPanic("Commit.Text", func() { c.Text(bad) })
	}
	mustPanic("Commit.FooterAt(-1)", func() { c.FooterAt(-1) })
	mustPanic("Commit.FooterAt(len)", func() { c.FooterAt(c.FooterCount()) })
	if !c.HasScope() || !c.HasBody() || !c.HasFooter() || !c.IndentSpan().IsZero() || c.Indent() != "" {
		t.Fatalf("unexpected presence flags: %#v", c)
	}
	if c.HeaderSeparator() != ": " || c.Text(c.HeaderSeparatorSpan()) != ": " {
		t.Fatalf("HeaderSeparator = %q", c.HeaderSeparator())
	}
	if c.Trailing() != "\r\n\n" || c.Text(c.TrailingSpan()) != "\r\n\n" {
		t.Fatalf("Trailing = %q", c.Trailing())
	}

	// BodyLines & FooterLines including early break.
	var bodyLines, footerLines []string
	for line := range c.BodyLines() {
		bodyLines = append(bodyLines, c.Text(line.Content))
	}
	for line := range c.FooterLines() {
		footerLines = append(footerLines, c.Text(line.Content))
	}
	if !reflect.DeepEqual(bodyLines, []string{"Body one", "Body two"}) || !reflect.DeepEqual(footerLines, []string{"Refs #10", "BREAKING CHANGE: drop v1"}) {
		t.Fatalf("lines mismatch: body=%q footer=%q", bodyLines, footerLines)
	}
	for range c.BodyLines() {
		break
	}

	// Absent BodyLines / FooterLines yield zero lines.
	minimal, _ := syntax.Parse("fix: bug")
	if minimal.HasScope() || minimal.HasBody() || minimal.HasFooter() || !minimal.TrailingSpan().IsZero() || minimal.Trailing() != "" {
		t.Fatal("minimal commit reported absent fields")
	}
	for range minimal.BodyLines() {
		t.Fatal("absent body yielded a line")
	}
	for range minimal.FooterLines() {
		t.Fatal("absent footer yielded a line")
	}

	// Footers iterator and Footer helpers.
	var tokens []string
	var breaking []bool
	for f := range c.Footers() {
		tokens = append(tokens, f.Token())
		breaking = append(breaking, f.IsBreaking())
		if c.Text(f.Span()) != f.Token()+c.Text(f.SeparatorSpan())+f.Value() {
			t.Fatalf("footer Span mismatch: %q", c.Text(f.Span()))
		}
	}
	if !reflect.DeepEqual(tokens, []string{"Refs", "BREAKING CHANGE"}) || !reflect.DeepEqual(breaking, []bool{false, true}) {
		t.Fatalf("footers mismatch: %v %v", tokens, breaking)
	}
	for range c.Footers() {
		break
	}

	// Locate and Commit.Position.
	if pos := syntax.Locate(input, -5); pos != (syntax.Position{Line: 1, Column: 1}) {
		t.Fatalf("clamped low position = %+v", pos)
	}
	if pos := c.Position(c.BodySpan().Start); pos != (syntax.Position{Line: 3, Column: 1}) {
		t.Fatalf("body start position = %+v", pos)
	}
	if pos := c.Position(c.FooterAt(1).ValueSpan().Start); pos != (syntax.Position{Line: 7, Column: 18}) {
		t.Fatalf("footer value position = %+v", pos)
	}
	if pos := syntax.Locate("a\nb", 99); pos != (syntax.Position{Line: 2, Column: 2}) {
		t.Fatalf("clamped high position = %+v", pos)
	}
}
