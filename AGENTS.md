# tokenizer

High-performance generic tokenizer (lexer) for Go. Parses any string, byte slice
or infinite `io.Reader` stream into a stream of typed tokens: user-defined tokens,
keywords, integers, floats, quoted strings (with escaping and embedded injections).
Used as the foundation for higher-level parsers and DSLs.

## Goal

Maximum parsing speed with minimum CPU and memory cost. The hot path is
per-byte: any extra work per input byte (rune decoding, map lookups, slice
scans, allocations) multiplies across the whole input. Design decisions that
keep it fast:

- zero allocations per token on the parse path (token pool, `token.value`
  slices point into the source buffer — no copies)
- lookup tables (`[256]`-arrays) instead of maps or slice scans in per-byte loops
- ASCII fast paths; full rune decoding only for bytes >= 0x80
- unsafe zero-copy string/byte conversions (`b2s`/`s2b` in helpers.go)

## Layout

- `tokenizer.go` — configuration: token definitions, string settings, token pool
- `parser.go` — the parser hot path (`parse`, `parseToken`, `parseKeyword`,
  `parseNumber`, `parseQuote`, `parseWhitespace`)
- `stream.go` — token stream iterator (in-memory and infinite modes)
- `token.go` — token accessors
- `bench_test.go` — benchmarks per workload (keywords, numbers, operators,
  strings, unicode, stream)

## Rules

- Everything written to the filesystem (code, comments, docs, commit messages)
  must be in English. Exceptions are allowed where content requires it, e.g.
  test data or examples.
- Talk to the user in the language of their messages.
- Use the `require` package (github.com/stretchr/testify/require) in tests.
- Always think about performance: this library competes on speed. Before
  changing the parse path, check the cost per input byte; verify with
  `make bench` (and pprof for anything non-trivial). Do not add allocations,
  map lookups or function calls to per-byte loops. Never trade hot-path
  performance for minor readability gains.
