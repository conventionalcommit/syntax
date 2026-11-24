package syntax_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

func TestInspectionRecovery(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		structured  bool
		codes       []syntax.ErrorCode
	}{
		{"valid", "feat: expose records\n\nRefs #9", true, nil},
		{"indent", " \tfeat: expose records", true, []syntax.ErrorCode{syntax.InvalidType}},
		{"missing space", "feat:expose records", true, []syntax.ErrorCode{syntax.InvalidSeparator}},
		{"blank description", "feat: \t", true, []syntax.ErrorCode{syntax.InvalidDescription}},
		{"missing space and description", "feat:", true, []syntax.ErrorCode{syntax.InvalidSeparator, syntax.InvalidDescription}},
		{"missing body gap", "feat: expose records\nBody text.", true, []syntax.ErrorCode{syntax.InvalidSectionSeparator}},
		{"missing footer gap", "feat: expose records\nRefs #9", true, []syntax.ErrorCode{syntax.InvalidSectionSeparator}},
		{"empty footer", "feat: expose records\n\nRefs #9\nEmpty: ", true, []syntax.ErrorCode{syntax.InvalidFooterValue}},
		{"empty breaking", "feat: expose records\n\nBREAKING CHANGE: ", true, []syntax.ErrorCode{syntax.InvalidFooterValue}},
		{"two empty footers", "feat: expose records\n\nRefs #9\nEmpty: \nNext: ", true, []syntax.ErrorCode{syntax.InvalidFooterValue, syntax.InvalidFooterValue}},
		{"unknown header", "feat records", false, []syntax.ErrorCode{syntax.InvalidType}},
		{"spaces only", "   ", false, []syntax.ErrorCode{syntax.InvalidType}},
		{"spaces and tabs only", " \t ", false, []syntax.ErrorCode{syntax.InvalidType}},
		{"whitespace header before body", "  \n\nBody text.", false, []syntax.ErrorCode{syntax.InvalidType}},
		{"missing colon", "feat", false, []syntax.ErrorCode{syntax.InvalidSeparator}},
		{"invalid scope", "feat(store: expose records", false, []syntax.ErrorCode{syntax.InvalidScope}},
		{"indent invalid scope", " feat(store: expose records", false, []syntax.ErrorCode{syntax.InvalidType, syntax.InvalidScope}},
		{"indent empty scope", " feat(): expose records", false, []syntax.ErrorCode{syntax.InvalidType, syntax.InvalidScope}},
		{"indent bang missing colon", " feat! expose records", false, []syntax.ErrorCode{syntax.InvalidType, syntax.InvalidSeparator}},
		{"indent missing colon and desc", " feat:\t", true, []syntax.ErrorCode{syntax.InvalidType, syntax.InvalidSeparator, syntax.InvalidDescription}},
		{"leading blank", "\nfeat: expose records", false, []syntax.ErrorCode{syntax.InvalidType}},
		{"invalid bytes", "feat: expose records\xff", false, []syntax.ErrorCode{syntax.InvalidEncoding}},
		{"bare cr", "feat: expose records\r", false, []syntax.ErrorCode{syntax.InvalidCharacter}},
	} {
		for _, nl := range []string{"\n", "\r\n"} {
			t.Run(tt.name+nl, func(t *testing.T) {
				input := strings.ReplaceAll(tt.input, "\n", nl)
				got := syntax.Inspect(input)
				if got.Structured != tt.structured || got.Commit.Message() != input || len(got.Diagnostics) != len(tt.codes) {
					t.Fatalf("inspection=%#v", got)
				}
				for i, code := range tt.codes {
					if got.Diagnostics[i].Code != code {
						t.Fatalf("diagnostic %d=%v", i, got.Diagnostics[i])
					}
				}
				if tt.structured && got.Commit.Type() != "feat" {
					t.Fatalf("type=%q", got.Commit.Type())
				}
				if !tt.structured && got.Commit.Type()+got.Commit.Scope()+got.Commit.Description()+got.Commit.Body()+got.Commit.Footer() != "" {
					t.Fatal("ambiguous fields exposed")
				}
				if len(tt.codes) == 0 {
					if !got.Valid() {
						t.Fatal("zero diagnostics should be Valid()")
					}
					strict, err := syntax.Parse(input)
					if err != nil || !reflect.DeepEqual(strict, got.Commit) {
						t.Fatal("strict and inspected fields differ")
					}
				} else if got.Valid() {
					t.Fatal("invalid commit reported Valid()")
				}
			})
		}
	}
}

func TestInspectionMultipleDiagnostics(t *testing.T) {
	input := "\tfeat:change\nBody\n\nBREAKING CHANGE: \nEmpty: "
	got := syntax.Inspect(input)
	want := []syntax.ParseError{
		{Code: syntax.InvalidType, Offset: 0},
		{Code: syntax.InvalidSeparator, Offset: 6},
		{Code: syntax.InvalidSectionSeparator, Offset: 13},
		{Code: syntax.InvalidFooterValue, Offset: 36},
		{Code: syntax.InvalidFooterValue, Offset: 44},
	}
	if !got.Structured || !reflect.DeepEqual(got.Diagnostics, want) {
		t.Fatalf("diagnostics=%v", got.Diagnostics)
	}
	c := got.Commit
	if c.Header() != "\tfeat:change" || c.Indent() != "\t" || c.HeaderSeparator() != ":" || c.Description() != "change" || c.Body() != "Body" || c.FooterCount() != 2 || c.HasHeaderBang() || !c.HasBreakingFooter() || c.BodyGap().BlankLines != 0 || c.FooterGap().BlankLines != 1 {
		t.Fatalf("unexpected recovered fields: %#v", c)
	}
	unstructured := syntax.Inspect("bad header").Commit
	if !unstructured.IndentSpan().IsZero() || !unstructured.HeaderSeparatorSpan().IsZero() {
		t.Fatal("unstructured commit exposed indent or separator")
	}
	last := c.FooterAt(1)
	if last.ValueSpan().Start != len(input) || last.ValueSpan().End != len(input) {
		t.Fatal("incorrect empty value position")
	}
	got.Diagnostics[0].Code = 0
	if c.Message() != input {
		t.Fatal("diagnostics changed the source")
	}
}

func TestInspectionRetainsLaterFooters(t *testing.T) {
	got := syntax.Inspect("feat: expose records\n\nRefs #9\nEmpty: \nNext: text")
	c := got.Commit
	if !got.Structured || len(got.Diagnostics) != 1 || c.FooterCount() != 3 {
		t.Fatalf("inspection=%#v", got)
	}
	first, empty, last := c.FooterAt(0), c.FooterAt(1), c.FooterAt(2)
	if first.Value() != "9" || empty.ValueSpan().Len() != 0 || last.Value() != "text" {
		t.Fatal("footer boundaries lost")
	}
}

var inspected syntax.Inspection

func TestInspectionAllocationBudgets(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  float64
	}{
		{"feat: expose records", 0},
		{"feat: expose records\n\nBody text.", 0},
		{"feat: expose records\n\nRefs #9", 1},
	} {
		got := testing.AllocsPerRun(100, func() { inspected = syntax.Inspect(tt.input) })
		if got != tt.want || !inspected.Structured || len(inspected.Diagnostics) != 0 {
			t.Fatalf("input=%q allocations=%g", tt.input, got)
		}
	}
}

func BenchmarkInspect(b *testing.B) {
	for _, tt := range []struct{ name, input string }{
		{"header", "feat: expose records"},
		{"footers", "feat: expose records\n\nRefs #9\nTrace: text"},
		{"recovery", "\tfeat:change\nBody\n\nBREAKING CHANGE: \nEmpty: "},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				inspected = syntax.Inspect(tt.input)
			}
		})
	}
}
