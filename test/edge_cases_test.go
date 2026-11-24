package syntax_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

func checkRejected(t testing.TB, input string, code syntax.ErrorCode, offset int, structured bool) {
	t.Helper()
	c, err := syntax.Parse(input)
	var pe *syntax.ParseError
	if !errors.As(err, &pe) || pe.Code != code || pe.Offset != offset || !reflect.DeepEqual(c, syntax.Commit{}) {
		t.Fatalf("error=%v, want code %d at byte %d and empty result", err, code, offset)
	}
	r := syntax.Inspect(input)
	if r.Structured != structured || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != code || r.Diagnostics[0].Offset != offset || r.Commit.Message() != input {
		t.Fatalf("unexpected inspection: %#v", r)
	}
	checkSourceRanges(t, r.Commit)
}

func TestMalformedHeaders(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		code        syntax.ErrorCode
		offset      int
		structured  bool
	}{
		{"empty", "", syntax.InvalidType, 0, false},
		{"space", " ", syntax.InvalidType, 0, false},
		{"tab", "\t", syntax.InvalidType, 0, false},
		{"only bang", "!", syntax.InvalidType, 0, false},
		{"only colon", ":", syntax.InvalidType, 0, false},
		{"punctuation type", "-: x", syntax.InvalidType, 1, false},
		{"missing type", "(f): x", syntax.InvalidType, 0, false},
		{"type with tab", "f\tx: y", syntax.InvalidType, 1, false},
		{"type with space", "f x: y", syntax.InvalidType, 1, false},
		{"type with closing paren", "f): x", syntax.InvalidType, 1, false},
		{"empty scope", "f(): x", syntax.InvalidScope, 2, false},
		{"nested scope", "f((s): x", syntax.InvalidScope, 2, false},
		{"double closing paren", "f(s)): x", syntax.InvalidSeparator, 4, false},
		{"missing close", "f(s", syntax.InvalidScope, 3, false},
		{"scope starts space", "f( s): x", syntax.InvalidScope, 2, false},
		{"scope ends space", "f(s ): x", syntax.InvalidScope, 3, false},
		{"scope tab", "f(s\t): x", syntax.InvalidScope, 3, false},
		{"scope colon", "f(s:x): y", syntax.InvalidScope, 3, false},
		{"scope bang", "f(s!): x", syntax.InvalidScope, 3, false},
		{"scope symbol", "f(©): x", syntax.InvalidScope, 2, false},
		{"double bang", "f(s)!!: x", syntax.InvalidSeparator, 5, false},
		{"space before colon", "f(s) : x", syntax.InvalidSeparator, 4, false},
		{"space after bang", "f(s)! : x", syntax.InvalidSeparator, 5, false},
		{"no colon", "f", syntax.InvalidSeparator, 1, false},
		{"newline before colon", "f\n: x", syntax.InvalidSeparator, 1, false},
		{"crlf before colon", "f\r\n: x", syntax.InvalidSeparator, 1, false},
		{"missing separator space", "f:x", syntax.InvalidSeparator, 2, true},
		{"tab separator", "f:\tx", syntax.InvalidSeparator, 2, true},
		{"double colon", "f:: x", syntax.InvalidSeparator, 2, true},
		{"colon ends header", "f:", syntax.InvalidSeparator, 2, true},
		{"empty description", "f: ", syntax.InvalidDescription, 3, true},
		{"unicode blank description", "f: \u00a0\u2003", syntax.InvalidDescription, 3, true},
		{"indented header", " f: x", syntax.InvalidType, 0, true},
		{"tabbed header", "\tf: x", syntax.InvalidType, 0, true},
		{"leading empty line", "\nf: x", syntax.InvalidType, 0, false},
		{"byte order mark", "\ufefff: x", syntax.InvalidType, 0, false},
		{"zero width type", "f\u200d: x", syntax.InvalidType, 1, false},
		{"emoji type", "🚀: x", syntax.InvalidType, 0, false},
		{"emoji scope", "f(🚀): x", syntax.InvalidScope, 2, false},
		{"emoji after scope word", "f(api🚀): x", syntax.InvalidScope, 5, false},
		{"gitmoji shortcode", ":sparkles: add entry", syntax.InvalidType, 0, false},
		{"unicode type separator", "f\u2028: x", syntax.InvalidType, 1, false},
		{"nonbreaking separator space", "f:\u00a0x", syntax.InvalidSeparator, 2, true},
		{"header nul", "f: x\x00", syntax.InvalidCharacter, 4, false},
	} {
		t.Run(tt.name, func(t *testing.T) { checkRejected(t, tt.input, tt.code, tt.offset, tt.structured) })
	}
}

func TestEveryByteInBody(t *testing.T) {
	prefix := "feat: retain bytes\n\n  body "
	for b := 0; b < 256; b++ {
		t.Run(fmt.Sprintf("%02x", b), func(t *testing.T) {
			input := prefix + string([]byte{byte(b)}) + " end"
			switch {
			case b == 0 || b == '\r':
				checkRejected(t, input, syntax.InvalidCharacter, len(prefix), false)
			case b >= 128:
				checkRejected(t, input, syntax.InvalidEncoding, len(prefix), false)
			default:
				checkMessage(t, input, expectedCommit{header: "feat: retain bytes", kind: "feat", description: "retain bytes", body: "  body " + string([]byte{byte(b)}) + " end"})
			}
		})
	}
}

func TestMalformedUTF8Positions(t *testing.T) {
	sequences := []string{"\x80", "\xbf", "\xc0\xaf", "\xc1\xbf", "\xe0\x80\x80", "\xed\xa0\x80", "\xf0\x80\x80\x80", "\xf4\x90\x80\x80", "\xf5\x80\x80\x80", "\xff", "\xc2", "\xe2\x82", "\xf0\x9f\x92"}
	for si, sequence := range sequences {
		for pi, prefix := range []string{"feat", "feat(", "feat: ", "feat: retain bytes\n\nbody ", "feat: retain bytes\n\nRefs", "feat: retain bytes\n\nRefs #9\nTrace: "} {
			t.Run(fmt.Sprintf("sequence%d/position%d", si, pi), func(t *testing.T) { checkRejected(t, prefix+sequence, syntax.InvalidEncoding, len(prefix), false) })
		}
	}
}

func TestUnicodeAndEmojiContent(t *testing.T) {
	for i, text := range []string{
		"🚀", "👩🏽‍💻", "👨‍👩‍👧‍👦", "🇮🇳🇯🇵", "1️⃣", "❤️", "🏳️‍🌈", "👍🏿", ":sparkles: add entry",
		"cafe\u0301", "مرحبا بالعالم", "नमस्ते दुनिया", "你好世界", "🌍\ufe0f\u200d🚀",
		"\u007f\u0080\u07ff\u0800\ud7ff\ue000\ufffd\uffff\U00010000\U0010ffff",
		"\u200b\u200d\u2060\ufeff", "\x1b[31mred\x1b[0m", "\u2028\u2029",
	} {
		t.Run(fmt.Sprintf("content%d", i), func(t *testing.T) {
			description := "retain " + text
			body := "  body " + text
			footer := "Trace: note " + text
			header := "feat: " + description
			checkMessage(t, header+"\n\n"+body+"\n\n"+footer, expectedCommit{header: header, kind: "feat", description: description, body: body, footer: footer, entries: []expectedNote{{"Trace", "note " + text, syntax.Colon}}})
		})
	}
}

func TestFooterLookalikesRemainText(t *testing.T) {
	for _, line := range []string{"Refs:#9", "Refs\t#9", "Refs : x", "Two Words: x", ": x", "-: x", "Ref_1: x", "Refs： x", "Refs:", "BREAKING CHANGE:", " BREAKING CHANGE: x", "breaking change: x", "BREAKING CHANGE #x", "> Refs #9", "* Refs #9", "https://example.org:8443/x", "12:34:56", "Refs #", "Refs: "} {
		t.Run(line, func(t *testing.T) {
			header := "feat: retain text"
			checkMessage(t, header+"\n\n"+line, expectedCommit{header: header, kind: "feat", description: "retain text", body: line})
			footer := "Trace: first\n" + line
			// Empty pairs inside established footers have their own error policy.
			if line == "Refs #" || line == "Refs: " {
				return
			}
			checkMessage(t, header+"\n\n"+footer, expectedCommit{header: header, kind: "feat", description: "retain text", footer: footer, entries: []expectedNote{{"Trace", "first\n" + line, syntax.Colon}}})
		})
	}
}

func TestInvalidFooterAndSectionBoundaries(t *testing.T) {
	header := "feat: retain text"
	for _, nl := range []string{"\n", "\r\n"} {
		for i, tail := range []string{"body", "Refs #9", " \nbody", "\t\nbody"} {
			t.Run(fmt.Sprintf("missing gap/%q/%d", nl, i), func(t *testing.T) {
				checkRejected(t, header+nl+strings.ReplaceAll(tail, "\n", nl), syntax.InvalidSectionSeparator, len(header)+len(nl), true)
			})
		}
		for _, pair := range []string{"Empty: ", "Empty #", "BREAKING CHANGE: ", "BREAKING-CHANGE: "} {
			for _, suffix := range []string{"", "\t\u2003", nl + "Next: value"} {
				t.Run(fmt.Sprintf("empty value/%q/%s/%q", nl, pair, suffix), func(t *testing.T) {
					prefix := header + nl + nl + "Refs #9" + nl
					checkRejected(t, prefix+pair+suffix, syntax.InvalidFooterValue, len(prefix)+len(pair), true)
				})
			}
		}
	}
}
