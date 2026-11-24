package syntax_test

import (
	"reflect"
	"testing"

	"github.com/conventionalcommit/syntax"
)

var (
	parsedCommit syntax.Commit
	parsedError  error
	inspected    syntax.Inspection
)

type expectedNote struct {
	token, value string
	separator    syntax.Separator
}

type expectedCommit struct {
	header, kind, scope, description, body, footer string
	entries                                        []expectedNote
	bang, breakingFooter                           bool
}

func checkMessage(t testing.TB, input string, want expectedCommit) syntax.Commit {
	t.Helper()
	c, err := syntax.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if c.Message() != input || c.Header() != want.header || c.Type() != want.kind || c.Scope() != want.scope || c.Description() != want.description || c.Body() != want.body || c.Footer() != want.footer || c.FooterCount() != len(want.entries) || c.HasHeaderBang() != want.bang || c.HasBreakingFooter() != want.breakingFooter || c.IsBreakingChange() != (want.bang || want.breakingFooter) {
		t.Fatalf("unexpected fields: type=%q scope=%q description bytes=%d body bytes=%d footer bytes=%d entries=%d markers=%v/%v", c.Type(), c.Scope(), len(c.Description()), len(c.Body()), len(c.Footer()), c.FooterCount(), c.HasHeaderBang(), c.HasBreakingFooter())
	}
	for i, entry := range want.entries {
		got := c.FooterAt(i)
		if got.Token() != entry.token || got.Value() != entry.value || got.Separator() != entry.separator {
			t.Fatalf("incorrect footer %d", i)
		}
	}
	inspection := syntax.Inspect(input)
	if !inspection.Structured || len(inspection.Diagnostics) != 0 || !reflect.DeepEqual(inspection.Commit, c) {
		t.Fatal("strict parsing and inspection disagree")
	}
	checkSourceRanges(t, c)
	return c
}

func checkSourceRanges(t testing.TB, c syntax.Commit) {
	t.Helper()
	source := c.Message()
	for _, field := range []struct {
		span syntax.Span
		text string
	}{
		{c.HeaderSpan(), c.Header()}, {c.TypeSpan(), c.Type()}, {c.ScopeSpan(), c.Scope()},
		{c.DescriptionSpan(), c.Description()}, {c.BodySpan(), c.Body()}, {c.FooterSpan(), c.Footer()},
	} {
		if field.span.Start < 0 || field.span.End < field.span.Start || field.span.End > len(source) {
			t.Fatal("field span outside source")
		}
		if source[field.span.Start:field.span.End] != field.text {
			t.Fatal("field text disagrees with its range")
		}
	}
	for _, span := range []syntax.Span{c.HeaderBangSpan(), c.BodyGap().Span, c.FooterGap().Span} {
		if span.Start < 0 || span.End < span.Start || span.End > len(source) {
			t.Fatal("fact span outside source")
		}
	}
	if c.HasHeaderBang() && source[c.HeaderBangSpan().Start:c.HeaderBangSpan().End] != "!" {
		t.Fatal("bang span lost punctuation")
	}
	lines := c.Lines(syntax.Span{Start: 0, End: len(source)})
	end, count := 0, 0
	for line, ok := lines.Next(); ok; line, ok = lines.Next() {
		if line.Content.Start != end || line.Content.End < end || line.Content.End != line.Ending.Start || line.Ending.End < line.Ending.Start || line.Ending.End > len(source) {
			t.Fatal("line spans are not contiguous source ranges")
		}
		end = line.Ending.End
		count++
		if count > len(source)+1 {
			t.Fatal("line iteration did not terminate")
		}
	}
	if end != len(source) {
		t.Fatal("line iteration lost source bytes")
	}
}
