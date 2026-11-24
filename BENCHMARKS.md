# Benchmarks

Cross-parser comparison running the same commit messages locally across Go and JavaScript Conventional Commits parsers.

## Environment and Parsers

- **CPU / OS**: `Intel(R) Core(TM) i7-10750H CPU @ 2.60GHz`, `linux/amd64`
- **Go runtime**: Go benchmarks (`go test -bench=. -benchmem`)
- **JS runtime**: Node.js `v24.21.0` (`process.hrtime.bigint()`, warmed before timing)

Tested parsers:

| Parser | Language | Configuration |
| :--- | :--- | :--- |
| `syntax` (`Parse`, `Inspect`) | Go | `github.com/conventionalcommit/syntax` |
| `parser` | Go | Previous `github.com/conventionalcommit/parser` lexer |
| `go-conventionalcommits` | Go | `github.com/leodido/go-conventionalcommits v0.13.0` (`TypesFreeForm`, pre-allocated `[]byte`) |
| `conventional-commits-parser` | JS | `conventional-commits-parser v7.1.4` (`CommitParser` instance reused) |
| `@commitlint/parse` | JS | `@commitlint/parse v21.2.3` (`parse`) |

## Corpus

| Case | Input Summary |
| :--- | :--- |
| `header` | `"feat: repack archive"` |
| `scope_bang` | `"feat(cache)!: replace archive index"` |
| `unicode` | `"feat(缓存): 保留原始内容"` |
| `body_footers` | Header, two body paragraphs, three footers (`Ref:`, `Date:`, `By:`) |
| `breaking_footer` | `"feat(api)!: drop v1\n\nBREAKING CHANGE: remove legacy endpoint\nRefs: #84"` |
| `body_16k` | Header, 400 body lines (~16 KB), one footer (`Refs: #84`) |
| `invalid` | `"feat(cache)! pack archive"` |

## Latency (`ns/op`, lower is better)

| Case | `syntax.Parse` | `syntax.Inspect` | `old parser` | `go-conventionalcommits` | `go-conventionalcommits` (strict UTF-8) | `conventional-commits-parser` | `@commitlint/parse` |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `header` | **83.4 ns** | 73.5 ns | 1,424 ns | 571.9 ns | 687.4 ns | 1,085.4 ns | 11,210.7 ns |
| `scope_bang` | **101.6 ns** | 103.6 ns | 1,804 ns | 658.1 ns | 704.4 ns | 1,149.3 ns | 11,429.9 ns |
| `unicode` | **159.1 ns** | 171.7 ns | 1,713 ns | 671.1 ns | 728.1 ns | 743.9 ns | 10,539.4 ns |
| `body_footers` | **723.7 ns** | 745.5 ns | 5,618 ns | 5,918 ns | 6,121 ns | 6,582.2 ns | 15,864.5 ns |
| `breaking_footer` | 384.0 ns | **373.8 ns** | 5,656 ns | 1,511 ns | 1,664 ns | 2,872.2 ns | 13,089.7 ns |
| `body_16k` | **24,816 ns** | 25,286 ns | 105,194 ns | 19,909,697 ns | 21,060,220 ns | 405,062.4 ns | 395,091.2 ns |
| `invalid` | **103.0 ns** | 134.2 ns | 1,445 ns | 781.1 ns | 809.5 ns | 982.4 ns | 9,919.1 ns |

## Memory and Allocations in Go (`B/op` and `allocs/op`)

| Case | `syntax.Parse` | `syntax.Inspect` | `old parser` | `go-conventionalcommits` |
| :--- | ---: | ---: | ---: | ---: |
| `header` | **0 B / 0** | **0 B / 0** | 1,552 B / 7 | 474 B / 14 |
| `scope_bang` | **0 B / 0** | **0 B / 0** | 1,552 B / 7 | 504 B / 16 |
| `unicode` | **0 B / 0** | **0 B / 0** | 1,552 B / 7 | 504 B / 16 |
| `body_footers` | **176 B / 1** | **176 B / 1** | 2,096 B / 56 | 3,880 B / 151 |
| `breaking_footer` | **112 B / 1** | **112 B / 1** | 2,176 B / 75 | 1,120 B / 37 |
| `body_16k` | **64 B / 1** | **64 B / 1** | 272,816 B / 20 | 135,776,044 B / 32,038 |
| `invalid` | **16 B / 1** | 32 B / 2 | 1,552 B / 7 | 552 B / 18 |

## Scaling (`syntax`)

From `go test -run '^$' -bench . -benchmem` in this repository:

| Benchmark | Size / Count | Speed | Memory |
| :--- | ---: | ---: | ---: |
| `BenchmarkBodyScaling` | 1 KB | 2,788 ns/op (`373 MB/s`) | 0 B / 0 allocs |
| `BenchmarkBodyScaling` | 16 KB | 40,891 ns/op (`401 MB/s`) | 0 B / 0 allocs |
| `BenchmarkBodyScaling` | 256 KB | 738,956 ns/op (`355 MB/s`) | 0 B / 0 allocs |
| `BenchmarkBodyScaling` | 1 MB | 2,747,126 ns/op (`382 MB/s`) | 0 B / 0 allocs |
| `BenchmarkFooterScaling` | 1 footer | 261.7 ns/op | 64 B / 1 alloc |
| `BenchmarkFooterScaling` | 1,024 footers | 105,360 ns/op | 57,344 B / 1 alloc |
| `BenchmarkFooterScaling` | 16,384 footers | 1,972,646 ns/op | 917,509 B / 1 alloc |
| `BenchmarkEmptyCandidateScaling` | 16,384 candidates | 1,547,447 ns/op | 0 B / 0 allocs |

## Takeaways

- `syntax.Parse` is **7.8x to 17.8x faster** than the previous `parser` package on short and multi-section commits, and **4.2x faster** with **4,262x less memory** on 16 KB bodies.
- Compared with `leodido/go-conventionalcommits`, `syntax.Parse` avoids per-transition allocations (`0` vs `14` to `151` allocs on short messages) and avoids quadratic string assembly on multi-line bodies (`24.8 us` / `64 B` vs `19.9 ms` / `129.5 MB` on `body_16k`).
- Compared with JavaScript parsers on Node 24, `syntax.Parse` is **4.7x to 16.3x faster** than `conventional-commits-parser` and **16x to 134x faster** than `@commitlint/parse` (which loads default preset configuration per call).
- Footer parsing allocates once (`[]Footer`, exact-sized via two-pass scan) regardless of whether the commit has 1 footer or 16,384 footers.
