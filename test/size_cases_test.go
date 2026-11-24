package syntax_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

func TestMinimalMessages(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  expectedCommit
	}{
		{"a: b", expectedCommit{header: "a: b", kind: "a", description: "b"}},
		{"f: x", expectedCommit{header: "f: x", kind: "f", description: "x"}},
		{"a: 🚀", expectedCommit{header: "a: 🚀", kind: "a", description: "🚀"}},
		{"修: x", expectedCommit{header: "修: x", kind: "修", description: "x"}},
		{"a!: b", expectedCommit{header: "a!: b", kind: "a", description: "b", bang: true}},
		{"a(b): c", expectedCommit{header: "a(b): c", kind: "a", scope: "b", description: "c"}},
		{"a: b\n\nc", expectedCommit{header: "a: b", kind: "a", description: "b", body: "c"}},
		{"a: b\n\nR #1", expectedCommit{header: "a: b", kind: "a", description: "b", footer: "R #1", entries: []expectedNote{{"R", "1", syntax.Hash}}}},
	} {
		t.Run(tt.input, func(t *testing.T) { checkMessage(t, tt.input, tt.want) })
	}
}

func TestMessageSizeBoundaries(t *testing.T) {
	sizes := []int{0, 1, 2, 15, 16, 17, 63, 64, 65, 255, 256, 4095, 4096, 65535, 65536, 1 << 20, 8 << 20}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("body/%d", size), func(t *testing.T) {
			body := strings.Repeat("x", size)
			header := "fix: size boundary"
			input := header
			if size > 0 {
				input += "\n\n" + body
			}
			checkMessage(t, input, expectedCommit{header: header, kind: "fix", description: "size boundary", body: body})
		})
		t.Run(fmt.Sprintf("description/%d", size), func(t *testing.T) {
			description := strings.Repeat("x", size)
			input := "fix: " + description
			if size == 0 {
				checkRejected(t, input, syntax.InvalidDescription, 5, true)
				return
			}
			checkMessage(t, input, expectedCommit{header: input, kind: "fix", description: description})
		})
		t.Run(fmt.Sprintf("footer-value/%d", size), func(t *testing.T) {
			value := strings.Repeat("x", size)
			header := "fix: size boundary"
			prefix := header + "\n\nRefs #1\nTrace: "
			if size == 0 {
				checkRejected(t, prefix, syntax.InvalidFooterValue, len(prefix), true)
				return
			}
			footer := "Refs #1\nTrace: " + value
			checkMessage(t, prefix+value, expectedCommit{header: header, kind: "fix", description: "size boundary", footer: footer, entries: []expectedNote{{"Refs", "1", syntax.Hash}, {"Trace", value, syntax.Colon}}})
		})
	}
	for _, size := range []int{1, 255, 4096, 65536} {
		t.Run(fmt.Sprintf("type/%d", size), func(t *testing.T) {
			kind := strings.Repeat("x", size)
			header := kind + ": size boundary"
			checkMessage(t, header, expectedCommit{header: header, kind: kind, description: "size boundary"})
		})
		t.Run(fmt.Sprintf("scope/%d", size), func(t *testing.T) {
			scope := strings.Repeat("x", size)
			header := "fix(" + scope + "): size boundary"
			checkMessage(t, header, expectedCommit{header: header, kind: "fix", scope: scope, description: "size boundary"})
		})
	}
}

func TestLargeOrderedFooterSets(t *testing.T) {
	for _, count := range []int{1, 2, 15, 16, 17, 64, 1000, 10000, 65536} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			var footer strings.Builder
			entries := make([]expectedNote, count)
			for i := 0; i < count; i++ {
				if i > 0 {
					footer.WriteByte('\n')
				}
				value := strconv.Itoa(i)
				footer.WriteString("Refs #" + value)
				entries[i] = expectedNote{"Refs", value, syntax.Hash}
			}
			header := "fix: retain order"
			text := footer.String()
			checkMessage(t, header+"\n\n"+text, expectedCommit{header: header, kind: "fix", description: "retain order", footer: text, entries: entries})
		})
	}
}

func TestLargeLineAndCandidateSets(t *testing.T) {
	t.Run("many body lines", func(t *testing.T) {
		body := strings.Repeat("  line\r\n", 100000) + "  final"
		header := "fix: retain lines"
		c := checkMessage(t, header+"\r\n\r\n"+body, expectedCommit{header: header, kind: "fix", description: "retain lines", body: body})
		it := c.Lines(c.BodySpan())
		count := 0
		for _, ok := it.Next(); ok; _, ok = it.Next() {
			count++
		}
		if count != 100001 {
			t.Fatalf("body lines=%d", count)
		}
	})
	t.Run("empty ordinary candidates", func(t *testing.T) {
		body := strings.Repeat("Empty: \n\n", 65536) + "Empty: "
		header := "fix: retain paragraphs"
		checkMessage(t, header+"\n\n"+body, expectedCommit{header: header, kind: "fix", description: "retain paragraphs", body: body})
	})
	t.Run("long value lookahead", func(t *testing.T) {
		value := "\n" + strings.Repeat("\n", 100000) + "  detail"
		footer := "Refs #1\nTrace: " + value
		header := "fix: retain continuation"
		checkMessage(t, header+"\n\n"+footer, expectedCommit{header: header, kind: "fix", description: "retain continuation", footer: footer, entries: []expectedNote{{"Refs", "1", syntax.Hash}, {"Trace", value, syntax.Colon}}})
	})
}

func TestLateInvalidBytes(t *testing.T) {
	for _, size := range []int{65536, 1 << 20, 8 << 20} {
		for _, tail := range []string{"\xff", "\x00", "\r"} {
			t.Run(fmt.Sprintf("%d/%q", size, tail), func(t *testing.T) {
				prefix := "fix: late byte\n\n" + strings.Repeat("x", size)
				code := syntax.InvalidCharacter
				if tail == "\xff" {
					code = syntax.InvalidEncoding
				}
				checkRejected(t, prefix+tail, code, len(prefix), false)
			})
		}
	}
}

func TestLargeInputAllocationBudgets(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		want        float64
	}{
		{"large body", "fix: allocation check\n\n" + strings.Repeat("x", 1<<20), 0},
		{"many lines", "fix: allocation check\n\n" + strings.Repeat("  line\n", 10000), 0},
		{"many footers", "fix: allocation check\n\n" + strings.Repeat("Refs #1\n", 10000), 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parseAllocs := testing.AllocsPerRun(3, func() { parsedCommit, parsedError = syntax.Parse(tt.input) })
			if parsedError != nil || parseAllocs != tt.want {
				t.Fatalf("parse error=%v allocations=%g", parsedError, parseAllocs)
			}
			inspectAllocs := testing.AllocsPerRun(3, func() { inspected = syntax.Inspect(tt.input) })
			if !inspected.Structured || len(inspected.Diagnostics) != 0 || inspectAllocs != tt.want {
				t.Fatalf("inspection allocations=%g diagnostics=%v", inspectAllocs, inspected.Diagnostics)
			}
		})
	}
}
