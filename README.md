[![PkgGoDev](https://pkg.go.dev/badge/github.com/conventionalcommit/syntax)](https://pkg.go.dev/github.com/conventionalcommit/syntax)

# syntax

A fast, zero-dependency Go parser for [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) that keeps exact source positions and original input bytes.

- Strict validation (`Parse`) and diagnostic error recovery (`Inspect`)
- Zero heap allocations for headers, bodies, gaps, and line scanning (1 allocation when footers exist)
- Zero-copy field accessors and exact byte `Span` / 1-based line `Position` offsets
- Lossless whitespace, line ending (`LF` / `CRLF`), and duplicate footer preservation

Requires Go 1.26 or later.

## Install

```sh
go get github.com/conventionalcommit/syntax
```

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	"github.com/conventionalcommit/syntax"
)

func main() {
	msg := "feat(api)!: replace query format\n\nKeep pagination consistent.\n\nRefs #84\nBREAKING CHANGE: drop v1 params"

	c, err := syntax.Parse(msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(c.Type())             // feat
	fmt.Println(c.Scope())            // api
	fmt.Println(c.Description())      // replace query format
	fmt.Println(c.Body())             // Keep pagination consistent.
	fmt.Println(c.IsBreakingChange()) // true

	for f := range c.Footers() {
		fmt.Printf("%s = %s (breaking: %v)\n", f.Token(), f.Value(), f.IsBreaking())
	}
}
```

## Error Positions

When `Parse` rejects a commit, it returns a zero `Commit` and a `*syntax.ParseError` carrying a machine-readable `Code` and byte `Offset`. Use `syntax.Locate` to turn that offset into a 1-based line and column:

```go
c, err := syntax.Parse(msg)
if err != nil {
	var pe *syntax.ParseError
	if errors.As(err, &pe) {
		pos := syntax.Locate(msg, pe.Offset)
		fmt.Printf("%s at line %d, col %d\n", pe.Code, pos.Line, pos.Column)
	}
	return err
}
```

## Linting and Recovery

For linters and tools that want all syntax errors plus any recoverable structure in malformed input, use `Inspect`:

```go
res := syntax.Inspect(" feat:missing space\nbody without blank line")

if res.Structured {
	// Fields could still be delimited cleanly.
	fmt.Println("type:", res.Commit.Type())
	fmt.Println("indent:", res.Commit.Indent())
	fmt.Println("body gap blank lines:", res.Commit.BodyGap().BlankLines)
}

for _, diag := range res.Diagnostics {
	pos := res.Commit.Position(diag.Offset)
	fmt.Printf("%d:%d %s\n", pos.Line, pos.Column, diag)
}
```

When `res.Valid()` is true (`res.Structured && len(res.Diagnostics) == 0`), `Parse` accepts the same message with the exact same fields.

## Notes on Behaviour

- **Zero-copy slicing**: `Commit` and `Footer` accessors slice the original input string without copying. Retaining a `Commit`, `Footer`, or returned substring can retain the original string in memory.
- **Line endings and gaps**: `BodyGap()` and `FooterGap()` report raw separator spans and blank line counts without stripping or normalizing `\n` and `\r\n`.
- **Footer detection**: Footers follow the Conventional Commits token rules (`Token: value`, `Token #value`, `BREAKING CHANGE: value`, and multiline continuations) at the end of the message. Duplicate tokens are preserved in source order.

## Performance

On typical single-line and multi-section messages, `syntax.Parse` runs in **~83 ns** (0 allocs) for headers and **~724 ns** (1 alloc) for full commits with body and footers, scaling linearly at ~380 MB/s on 1 MB bodies.

See [BENCHMARKS.md](BENCHMARKS.md) for local side-by-side numbers comparing `syntax`, the previous `parser`, `leodido/go-conventionalcommits`, `conventional-commits-parser`, and `@commitlint/parse`.

## API Documentation

Full package documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/conventionalcommit/syntax).

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go test -run '^$' -bench . -benchmem
go test -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s ./test
```

## Licence

[MIT](LICENSE.md)
