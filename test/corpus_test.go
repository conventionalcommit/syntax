package syntax_test

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

var corpusTypes = []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert", "release", "FeAt", "ci/test", "修复", "123"}
var corpusScopes = []string{"", "api", "web/server", "cache-v2", "cache_store", "data store", "缓存", "cafe\u0301"}
var corpusDescriptions = []string{"x", "repair record", "🚀 ship", "🐛 repair 👩🏽‍💻", "family 👨‍👩‍👧‍👦", "你好 नमस्ते مرحبا", "  retain : ! (#9)  "}

func TestHeaderCombinations(t *testing.T) {
	for ti, kind := range corpusTypes {
		for si, scope := range corpusScopes {
			for _, bang := range []bool{false, true} {
				for di, description := range corpusDescriptions {
					for ni, ending := range []string{"", "\n", "\r\n"} {
						t.Run(fmt.Sprintf("type%d/scope%d/bang%v/text%d/end%d", ti, si, bang, di, ni), func(t *testing.T) {
							header := kind
							if scope != "" {
								header += "(" + scope + ")"
							}
							if bang {
								header += "!"
							}
							header += ": " + description
							checkMessage(t, header+ending, expectedCommit{header: header, kind: kind, scope: scope, description: description, bang: bang})
						})
					}
				}
			}
		}
	}
}

func TestSectionCombinations(t *testing.T) {
	bodies := []string{
		"", "plain text", "first\nsecond", "first\n\nsecond", "🚀 deploy 👩🏽‍💻",
		"```go\nprintln(42)\n```", "paragraph\nRefs #8", "  indented\n> Refs #8",
		"https://example.org/path?q=a:b", "text  \nline\t", "paragraph\nBREAKING CHANGE: ordinary text",
	}
	footers := []struct {
		text     string
		entries  []expectedNote
		breaking bool
	}{
		{},
		{"Refs #1", []expectedNote{{"Refs", "1", syntax.Hash}}, false},
		{"Trace: #1", []expectedNote{{"Trace", "#1", syntax.Colon}}, false},
		{"Reviewed-by: Sam\nRefs #2", []expectedNote{{"Reviewed-by", "Sam", syntax.Colon}, {"Refs", "2", syntax.Hash}}, false},
		{"Trace: first\n second\nRefs #3", []expectedNote{{"Trace", "first\n second", syntax.Colon}, {"Refs", "3", syntax.Hash}}, false},
		{"BREAKING CHANGE: remove alias", []expectedNote{{"BREAKING CHANGE", "remove alias", syntax.Colon}}, true},
		{"BREAKING-CHANGE: remove alias", []expectedNote{{"BREAKING-CHANGE", "remove alias", syntax.Colon}}, true},
		{"Refs #1\nRefs #1", []expectedNote{{"Refs", "1", syntax.Hash}, {"Refs", "1", syntax.Hash}}, false},
		{"追跡: 値🚀", []expectedNote{{"追跡", "値🚀", syntax.Colon}}, false},
		{"Trace: first\n\nlast", []expectedNote{{"Trace", "first\n\nlast", syntax.Colon}}, false},
		{"Trace: \n value", []expectedNote{{"Trace", "\n value", syntax.Colon}}, false},
	}
	for bi, body := range bodies {
		for fi, footer := range footers {
			for ni, nl := range []string{"\n", "\r\n"} {
				for _, blanks := range []int{1, 3} {
					for _, bang := range []bool{false, true} {
						for ei, tail := range []string{"", nl + nl} {
							t.Run(fmt.Sprintf("body%d/footer%d/newline%d/gap%d/bang%v/tail%d", bi, fi, ni, blanks, bang, ei), func(t *testing.T) {
								convert := func(s string) string { return strings.ReplaceAll(s, "\n", nl) }
								header := "feat(api)"
								if bang {
									header += "!"
								}
								header += ": preserve 🚀 records"
								gap := strings.Repeat(nl, blanks+1)
								input := header
								if body != "" {
									input += gap + convert(body)
								}
								if footer.text != "" {
									input += gap + convert(footer.text)
								}
								input += tail
								entries := make([]expectedNote, len(footer.entries))
								for i, e := range footer.entries {
									entries[i] = expectedNote{e.token, convert(e.value), e.separator}
								}
								c := checkMessage(t, input, expectedCommit{header: header, kind: "feat", scope: "api", description: "preserve 🚀 records", body: convert(body), footer: convert(footer.text), entries: entries, bang: bang, breakingFooter: footer.breaking})
								if body != "" && c.BodyGap().BlankLines != blanks {
									t.Fatal("incorrect body gap")
								}
								if footer.text != "" && c.FooterGap().BlankLines != blanks {
									t.Fatal("incorrect footer gap")
								}
							})
						}
					}
				}
			}
		}
	}
}

func TestMixedEndingCombinations(t *testing.T) {
	for mask := 0; mask < 64; mask++ {
		for ti, tail := range []string{"", "\n", "\r\n"} {
			t.Run(fmt.Sprintf("mask%d/tail%d", mask, ti), func(t *testing.T) {
				endings := make([]string, 6)
				for i := range endings {
					endings[i] = "\n"
					if mask&(1<<i) != 0 {
						endings[i] = "\r\n"
					}
				}
				lines := []string{"feat: mixed endings", "", "first", "second", "", "Refs #1", "Refs #2"}
				var input strings.Builder
				for i, line := range lines {
					input.WriteString(line)
					if i < 6 {
						input.WriteString(endings[i])
					} else {
						input.WriteString(tail)
					}
				}
				body := "first" + endings[2] + "second"
				footer := "Refs #1" + endings[5] + "Refs #2"
				c := checkMessage(t, input.String(), expectedCommit{header: lines[0], kind: "feat", description: "mixed endings", body: body, footer: footer, entries: []expectedNote{{"Refs", "1", syntax.Hash}, {"Refs", "2", syntax.Hash}}})
				if c.BodyGap().BlankLines != 1 || c.FooterGap().BlankLines != 1 {
					t.Fatal("mixed ending gap count changed")
				}
			})
		}
	}
}

func TestIndentedHeaderRecoveryMatrix(t *testing.T) {
	for ti, kind := range corpusTypes {
		for si, scope := range corpusScopes {
			for _, bang := range []bool{false, true} {
				for pi, prefix := range []string{" ", "\t", " \t", "\u00a0", "\n"} {
					t.Run(fmt.Sprintf("type%d/scope%d/bang%v/prefix%d", ti, si, bang, pi), func(t *testing.T) {
						header := kind
						if scope != "" {
							header += "(" + scope + ")"
						}
						if bang {
							header += "!"
						}
						header += ": retain 🚀 records"
						input := prefix + header
						structured := pi < 3
						checkRejected(t, input, syntax.InvalidType, 0, structured)
						if structured {
							c := syntax.Inspect(input).Commit
							if c.Header() != input || c.Type() != kind || c.Scope() != scope || c.Description() != "retain 🚀 records" || c.HasHeaderBang() != bang || c.TypeSpan().Start != len(prefix) {
								t.Fatal("recovery changed a known field or source offset")
							}
							if scope != "" && c.ScopeSpan().Start != len(prefix)+len(kind)+1 {
								t.Fatal("incorrect recovered scope offset")
							}
							if bang && input[c.HeaderBangSpan().Start:c.HeaderBangSpan().End] != "!" {
								t.Fatal("incorrect recovered bang span")
							}
						}
					})
				}
			}
		}
	}
}

func TestWhitespaceOnlyBodyContent(t *testing.T) {
	for _, space := range []string{" ", "\t", " \t", "\u00a0", "\u2003"} {
		for _, nl := range []string{"\n", "\r\n"} {
			t.Run(fmt.Sprintf("%q/%q", space, nl), func(t *testing.T) {
				header := "feat: retain whitespace"
				checkMessage(t, header+nl+nl+space+nl, expectedCommit{header: header, kind: "feat", description: "retain whitespace", body: space})
				body := "first" + nl + space + nl + "Refs #9"
				checkMessage(t, header+nl+nl+body, expectedCommit{header: header, kind: "feat", description: "retain whitespace", body: body})
			})
		}
	}
}

func TestUnicodeNames(t *testing.T) {
	for _, name := range []string{"é", "e\u0301", "Ω", "Ж", "क", "中", "𐐀", "𝟙", "a・b", "web,api", "@web/core", "\u0301x"} {
		t.Run(name, func(t *testing.T) {
			header := name + "(" + name + ")!: retain text"
			checkMessage(t, header, expectedCommit{header: header, kind: name, scope: name, description: "retain text", bang: true})
		})
	}
}

func TestManyRecoverableFooterErrors(t *testing.T) {
	for _, count := range []int{1, 64, 1024, 16384} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			prefix := "feat: retain diagnostics\n\nRefs #1\n"
			input := prefix + strings.Repeat("Empty: \n", count-1) + "Empty: "
			c, err := syntax.Parse(input)
			var pe *syntax.ParseError
			first := len(prefix) + len("Empty: ")
			if c.Message() != "" || !errors.As(err, &pe) || pe.Code != syntax.InvalidFooterValue || pe.Offset != first {
				t.Fatalf("strict result error=%v", err)
			}
			inspection := syntax.Inspect(input)
			if !inspection.Structured || len(inspection.Diagnostics) != count || inspection.Commit.FooterCount() != count+1 {
				t.Fatalf("diagnostics=%d footers=%d", len(inspection.Diagnostics), inspection.Commit.FooterCount())
			}
			for i, diagnostic := range inspection.Diagnostics {
				want := first + i*len("Empty: \n")
				entry := inspection.Commit.FooterAt(i + 1)
				if diagnostic.Code != syntax.InvalidFooterValue || diagnostic.Offset != want || entry.ValueSpan().Start != want || entry.ValueSpan().End != want {
					t.Fatalf("incorrect error or empty value at entry %d", i)
				}
			}
			checkSourceRanges(t, inspection.Commit)
		})
	}
}
