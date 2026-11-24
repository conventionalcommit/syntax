package syntax_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

// Cases are independently written from the numbered specification requirements.
// Semantic choices of type (requirements 2 and 3) remain the caller's duty.
func TestSpecification(t *testing.T) {
	type entry struct{ token, value string }
	for _, tt := range []struct {
		name, message, kind, scope, description, body string
		breaking                                      bool
		entries                                       []entry
	}{
		{name: "r01_r05_minimal", message: "deploy: rotate certificates", kind: "deploy", description: "rotate certificates"},
		{name: "r02_feature", message: "feat: expose metrics", kind: "feat", description: "expose metrics"},
		{name: "r03_fix", message: "fix: repair counter", kind: "fix", description: "repair counter"},
		{name: "r04_scope", message: "fix(queue): repair counter", kind: "fix", scope: "queue", description: "repair counter"},
		{name: "r06_r07_body", message: "ops: rotate certificates\n\nFirst paragraph.\nA second line.\n\nAnother paragraph.", kind: "ops", description: "rotate certificates", body: "First paragraph.\nA second line.\n\nAnother paragraph."},
		{name: "r08_colon", message: "ops: rotate certificates\n\nTicket: z-19", kind: "ops", description: "rotate certificates", entries: []entry{{"Ticket", "z-19"}}},
		{name: "r08_hash", message: "ops: rotate certificates\n\nTicket #19", kind: "ops", description: "rotate certificates", entries: []entry{{"Ticket", "19"}}},
		{name: "r09_hyphen", message: "ops: rotate certificates\n\nVerified-by: Sam", kind: "ops", description: "rotate certificates", entries: []entry{{"Verified-by", "Sam"}}},
		{name: "r10_pair_terminates_value", message: "ops: rotate certificates\n\nTicket: z-19\ncontinued:without separator\nTicket #20", kind: "ops", description: "rotate certificates", entries: []entry{{"Ticket", "z-19\ncontinued:without separator"}, {"Ticket", "20"}}},
		{name: "r10_value_has_spaces_newlines", message: "ops: rotate certificates\n\nTicket: first line  \n\n  continued\nVerified-by: Sam", kind: "ops", description: "rotate certificates", entries: []entry{{"Ticket", "first line  \n\n  continued"}, {"Verified-by", "Sam"}}},
		{name: "r11_r12_footer_breaking", message: "ops: rotate certificates\n\nBREAKING CHANGE: replace trust root", kind: "ops", description: "rotate certificates", breaking: true, entries: []entry{{"BREAKING CHANGE", "replace trust root"}}},
		{name: "r13_bang_only", message: "ops!: replace trust root", kind: "ops", description: "replace trust root", breaking: true},
		{name: "r13_scope_bang", message: "ops(tls)!: replace trust root", kind: "ops", scope: "tls", description: "replace trust root", breaking: true},
		{name: "r13_both_indicators", message: "ops!: replace trust root\n\nBREAKING CHANGE: renew all certificates", kind: "ops", description: "replace trust root", breaking: true, entries: []entry{{"BREAKING CHANGE", "renew all certificates"}}},
		{name: "r14_custom_type", message: "housekeeping: rotate certificates", kind: "housekeeping", description: "rotate certificates"},
		{name: "r15_case_preserved", message: "FeAt(TLS): Expose Metrics\n\ntIcKeT: Z-19", kind: "FeAt", scope: "TLS", description: "Expose Metrics", entries: []entry{{"tIcKeT", "Z-19"}}},
		{name: "r15_breaking_case_exception", message: "ops: rotate certificates\n\nbreaking-change: ordinary note", kind: "ops", description: "rotate certificates", entries: []entry{{"breaking-change", "ordinary note"}}},
		{name: "r16_hyphen_equivalence", message: "ops: rotate certificates\n\nBREAKING-CHANGE: replace trust root", kind: "ops", description: "rotate certificates", breaking: true, entries: []entry{{"BREAKING-CHANGE", "replace trust root"}}},
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(tt.name+newline, func(t *testing.T) {
				convert := func(s string) string { return strings.ReplaceAll(s, "\n", newline) }
				input := convert(tt.message)
				c, err := syntax.Parse(input)
				if err != nil {
					t.Fatal(err)
				}
				if c.Message() != input || c.Type() != tt.kind || c.Scope() != tt.scope || c.Description() != tt.description || c.Body() != convert(tt.body) || c.IsBreakingChange() != tt.breaking || c.FooterCount() != len(tt.entries) {
					t.Fatalf("unexpected parsed fields: %#v", c)
				}
				for i, want := range tt.entries {
					got := c.FooterAt(i)
					if got.Token() != want.token || got.Value() != convert(want.value) {
						t.Fatalf("footer %d: %#v", i, got)
					}
				}
			})
		}
	}
}

func TestPackageChoices(t *testing.T) {
	for _, tt := range []struct {
		input, kind, scope, body string
		entries                  int
	}{
		{"fe\u0301at(cafe\u0301): retain spelling", "fe\u0301at", "cafe\u0301", "", 0},
		{"release.v2(queue/store): retain punctuation", "release.v2", "queue/store", "", 0},
		{"ops: rebuild\n\nTicket_Ref: body text", "ops", "", "Ticket_Ref: body text", 0},
		{"ops: rebuild\n\n--: body text", "ops", "", "--: body text", 0},
		{"ops: rebuild\n\nTicket: \n\ntext", "ops", "", "", 1},
		{"ops: rebuild\n\nbody\n \nTicket: text", "ops", "", "body\n \nTicket: text", 0},
		{"ops: rebuild\n\nbody\n\n\nTicket: text", "ops", "", "body", 1},
		{"ops: rebuild\n\nTicket: text\n\tNext: continuation", "ops", "", "", 1},
		{"ops: rebuild\n\nbody\nBREAKING CHANGE: body text", "ops", "", "body\nBREAKING CHANGE: body text", 0},
		{"ops: rebuild\n\nbody\n\nmore\n\nTicket #8", "ops", "", "body\n\nmore", 1},
		{"ops: rebuild\n\nbody\t  \n\n", "ops", "", "body\t  ", 0},
		{"ops: rebuild\n\nT\u0301icket: value", "ops", "", "", 1},
		{"ops: rebuild\n\nBREAKING CHANGE:", "ops", "", "BREAKING CHANGE:", 0},
		{"ops: rebuild\n\nRefs #1\n\n\nTrace: a", "ops", "", "", 2},
	} {
		t.Run(tt.input, func(t *testing.T) {
			c, err := syntax.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if c.Type() != tt.kind || c.Scope() != tt.scope || c.Body() != tt.body || c.FooterCount() != tt.entries || c.IsBreakingChange() {
				t.Fatalf("unexpected fields: %#v", c)
			}
		})
	}
}

func TestByteOffsetsAndErrorPrecedence(t *testing.T) {
	for _, tt := range []struct {
		input  string
		code   syntax.ErrorCode
		offset int
	}{
		{"修复( ): change", syntax.InvalidScope, 7},
		{"修复: \t", syntax.InvalidDescription, 8},
		{"ops: rebuild\n\nTicket:\ttext", 0, 0}, // No footer separator; ordinary body.
		{"ops: rebuild\n\nbody\xff", syntax.InvalidEncoding, 18},
		{"bad header\x00\xff", syntax.InvalidCharacter, 10},
		{"bad header\xff\x00", syntax.InvalidEncoding, 10},
		{"bad header\r\xff", syntax.InvalidCharacter, 10},
		{"ops: rebuild\n\nbody\r", syntax.InvalidCharacter, 18},
		{"ops: rebuild\n\nbody\x00", syntax.InvalidCharacter, 18},
	} {
		t.Run(tt.input, func(t *testing.T) {
			_, err := syntax.Parse(tt.input)
			if tt.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var pe *syntax.ParseError
			if !errors.As(err, &pe) || pe.Code != tt.code || pe.Offset != tt.offset {
				t.Fatalf("error=%v, want code %d at byte %d", err, tt.code, tt.offset)
			}
		})
	}
}

func TestFooterAtBounds(t *testing.T) {
	var zero syntax.Commit
	func() {
		defer func() {
			if recover() == nil {
				t.Error("zero Commit.FooterAt(0) did not panic")
			}
		}()
		zero.FooterAt(0)
	}()

	c, err := syntax.Parse("ops: rebuild\n\nTicket #8")
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{-1, 1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("index %d did not panic", index)
				}
			}()
			c.FooterAt(index)
		}()
	}
}

func TestParseErrorFormatting(t *testing.T) {
	var nilErr *syntax.ParseError
	if nilErr.Error() != "<nil>" {
		t.Fatalf("nil Error() = %q", nilErr.Error())
	}
	for _, tc := range []struct {
		code syntax.ErrorCode
		want string
	}{
		{syntax.InvalidEncoding, "syntax: invalid UTF-8 at byte 3"},
		{syntax.InvalidCharacter, "syntax: invalid character at byte 3"},
		{syntax.InvalidType, "syntax: invalid type at byte 3"},
		{syntax.InvalidScope, "syntax: invalid scope at byte 3"},
		{syntax.InvalidSeparator, "syntax: expected colon and space at byte 3"},
		{syntax.InvalidDescription, "syntax: expected nonblank description at byte 3"},
		{syntax.InvalidSectionSeparator, "syntax: expected blank line before content at byte 3"},
		{syntax.InvalidFooterValue, "syntax: expected nonblank footer value at byte 3"},
		{syntax.ErrorCode(99), "syntax: invalid input at byte 3"},
	} {
		err := &syntax.ParseError{Code: tc.code, Offset: 3}
		if got := err.Error(); got != tc.want {
			t.Fatalf("code %d: got %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestStringAndTextHelpers(t *testing.T) {
	for _, tc := range []struct {
		code syntax.ErrorCode
		want string
	}{
		{syntax.InvalidEncoding, "InvalidEncoding"},
		{syntax.InvalidCharacter, "InvalidCharacter"},
		{syntax.InvalidType, "InvalidType"},
		{syntax.InvalidScope, "InvalidScope"},
		{syntax.InvalidSeparator, "InvalidSeparator"},
		{syntax.InvalidDescription, "InvalidDescription"},
		{syntax.InvalidSectionSeparator, "InvalidSectionSeparator"},
		{syntax.InvalidFooterValue, "InvalidFooterValue"},
		{syntax.ErrorCode(99), "ErrorCode(99)"},
	} {
		if got := tc.code.String(); got != tc.want {
			t.Fatalf("ErrorCode(%d).String() = %q, want %q", tc.code, got, tc.want)
		}
	}
	for _, tc := range []struct {
		sep  syntax.Separator
		want string
	}{
		{syntax.Colon, "Colon"},
		{syntax.Hash, "Hash"},
		{syntax.Separator(9), "Separator(9)"},
	} {
		if got := tc.sep.String(); got != tc.want {
			t.Fatalf("Separator(%d).String() = %q, want %q", tc.sep, got, tc.want)
		}
	}
	c, err := syntax.Parse("ops: rebuild\n\nTicket #8")
	if err != nil {
		t.Fatal(err)
	}
	entry := c.FooterAt(0)
	if c.Text(entry.TokenSpan()) != "Ticket" || c.Text(entry.ValueSpan()) != "8" || c.Text(syntax.Span{}) != "" {
		t.Fatalf("unexpected Text results: %q %q", entry.Token(), entry.Value())
	}
	if entry.TokenSpan().Len() != 6 || (syntax.Span{}).Len() != 0 {
		t.Fatal("unexpected Span.Len()")
	}
}
