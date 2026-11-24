package syntax_test

import (
	"fmt"

	"github.com/conventionalcommit/syntax"
)

func ExampleParse() {
	c, err := syntax.Parse("build(cache)!: replace archive index\n\nRefs #84")
	if err != nil {
		panic(err)
	}
	fmt.Println(c.Type(), c.Scope(), c.IsBreakingChange())
	entry := c.FooterAt(0)
	fmt.Println(entry.Token())
	fmt.Println(entry.Value())
	// Output:
	// build cache true
	// Refs
	// 84
}

func ExampleInspect() {
	r := syntax.Inspect("feat:reload\nBody text.")
	fmt.Println(r.Structured, len(r.Diagnostics))
	fmt.Println(r.Commit.Type(), r.Commit.Description())
	fmt.Println(r.Commit.BodyGap().BlankLines)
	// Output:
	// true 2
	// feat reload
	// 0
}

func ExampleLines() {
	it := syntax.Lines("a\r\nb\n")
	for line, ok := it.Next(); ok; line, ok = it.Next() {
		fmt.Println(line.Content.Len(), line.Ending.Len())
	}
	// Output:
	// 1 2
	// 1 1
	// 0 0
}
