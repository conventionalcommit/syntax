package syntax_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/conventionalcommit/syntax"
)

var parsedCommit syntax.Commit
var parsedError error

func BenchmarkParse(b *testing.B) {
	for _, tt := range []struct{ name, input string }{
		{"header", "build: repack archive"},
		{"scope_bang", "build(cache)!: replace archive index"},
		{"unicode", "修复(缓存): 保留原始内容"},
		{"body", "build: repack archive\n\nKeep the newest records.\n\nDiscard obsolete entries."},
		{"footers", "build: repack archive\n\nKeep the newest records.\n\nTrace: task-84\nChecked-by: Lee\nRefs #84"},
		{"crlf", "build(cache): repack\r\n\r\nKeep records.\r\n\r\nRefs #84\r\n"},
		{"invalid", "build(cache)! pack archive"},
		{"many_footers", "build: repack\n\n" + strings.Repeat("Refs #84\n", 100)},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tt.input)))
			for i := 0; i < b.N; i++ {
				parsedCommit, parsedError = syntax.Parse(tt.input)
			}
		})
	}
}

func BenchmarkBodyScaling(b *testing.B) {
	for _, size := range []int{1024, 16 * 1024, 256 * 1024, 1024 * 1024} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			input := "build: repack\n\n" + strings.Repeat("x", size)
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parsedCommit, parsedError = syntax.Parse(input)
			}
		})
	}
}

func TestAllocationBudgets(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		want        float64
	}{
		{"header", "build: repack", 0},
		{"scoped", "build(cache)!: repack", 0},
		{"body", "build: repack\n\nA paragraph.\n\nAnother paragraph.", 0},
		{"footers", "build: repack\n\nRefs #84\nTrace: first\nTrace: second", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := testing.AllocsPerRun(100, func() { parsedCommit, parsedError = syntax.Parse(tt.input) })
			if parsedError != nil {
				t.Fatal(parsedError)
			}
			if got != tt.want {
				t.Fatalf("allocations = %g, want %g", got, tt.want)
			}
		})
	}
}

func BenchmarkFooterScaling(b *testing.B) {
	for _, count := range []int{1, 32, 1024, 16384} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			input := "ops: rebuild\n\n" + strings.Repeat("Ticket #8\n", count)
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parsedCommit, parsedError = syntax.Parse(input)
			}
			if parsedError != nil || parsedCommit.FooterCount() != count {
				b.Fatal("incorrect footer count")
			}
		})
	}
}

func BenchmarkEmptyCandidateScaling(b *testing.B) {
	for _, count := range []int{1, 32, 1024, 16384} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			input := "ops: rebuild\n\n" + strings.Repeat("Ticket: \n\n", count)
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parsedCommit, parsedError = syntax.Parse(input)
			}
			if parsedError != nil || parsedCommit.FooterCount() != 0 {
				b.Fatal("incorrect candidate handling")
			}
		})
	}
}
