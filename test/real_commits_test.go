package syntax_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

type realCommitCase struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	URL        string `json:"url"`
	Selection  string `json:"selection"`
	Message    string `json:"message"`
	Expected   struct {
		Kind        string      `json:"type"`
		Scope       string      `json:"scope"`
		Description syntax.Span `json:"description"`
		Body        syntax.Span `json:"body"`
		Footer      syntax.Span `json:"footer"`
		Error       string      `json:"error"`
		Offset      int         `json:"offset"`
		Structured  bool        `json:"structured"`
		Entries     []struct {
			Token     syntax.Span `json:"token"`
			Value     syntax.Span `json:"value"`
			Separator string      `json:"separator"`
		} `json:"entries"`
	} `json:"expected"`
}

func readRealCommits(t testing.TB) []realCommitCase {
	t.Helper()
	data, err := os.ReadFile("testdata/real_commits.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []realCommitCase
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("empty real commit corpus")
	}
	return fixtures
}

func TestRealWorldCommits(t *testing.T) {
	for _, fixture := range readRealCommits(t) {
		t.Run(fixture.Repository+"/"+fixture.Commit+"/"+fixture.Selection, func(t *testing.T) {
			if len(fixture.Commit) != 40 || fixture.URL != "https://github.com/"+fixture.Repository+"/commit/"+fixture.Commit {
				t.Fatal("missing pinned provenance")
			}
			if fixture.Selection != "subject" && fixture.Selection != "full" {
				t.Fatal("unknown source selection")
			}
			if fixture.Selection == "subject" && strings.Contains(fixture.Message, "\n") {
				t.Fatal("subject fixture contains body text")
			}
			want := fixture.Expected
			if want.Error != "" {
				code, ok := map[string]syntax.ErrorCode{
					"InvalidEncoding":         syntax.InvalidEncoding,
					"InvalidCharacter":        syntax.InvalidCharacter,
					"InvalidType":             syntax.InvalidType,
					"InvalidScope":            syntax.InvalidScope,
					"InvalidSeparator":        syntax.InvalidSeparator,
					"InvalidDescription":      syntax.InvalidDescription,
					"InvalidSectionSeparator": syntax.InvalidSectionSeparator,
					"InvalidFooterValue":      syntax.InvalidFooterValue,
				}[want.Error]
				if !ok {
					t.Fatalf("unknown fixture error category %q", want.Error)
				}
				checkRejected(t, fixture.Message, code, want.Offset, want.Structured)
				return
			}
			text := func(span syntax.Span) string { return fixture.Message[span.Start:span.End] }
			entries := make([]expectedNote, len(want.Entries))
			for i, entry := range want.Entries {
				sep, ok := map[string]syntax.Separator{
					"colon": syntax.Colon,
					"hash":  syntax.Hash,
				}[entry.Separator]
				if !ok {
					t.Fatalf("unknown fixture separator %q", entry.Separator)
				}
				entries[i] = expectedNote{text(entry.Token), text(entry.Value), sep}
			}
			header, _, _ := strings.Cut(fixture.Message, "\n")
			c := checkMessage(t, fixture.Message, expectedCommit{header: header, kind: want.Kind, scope: want.Scope, description: text(want.Description), body: text(want.Body), footer: text(want.Footer), entries: entries})
			if c.DescriptionSpan() != want.Description || c.BodySpan() != want.Body || c.FooterSpan() != want.Footer {
				t.Fatal("source positions differ from reviewed fixture")
			}
		})
	}
}
