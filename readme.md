# Tokenizer

[![Build Status](https://github.com/bzick/tokenizer/actions/workflows/tokenizer.yml/badge.svg)](https://github.com/bzick/tokenizer/actions/workflows/tokenizer.yml)
[![codecov](https://codecov.io/gh/bzick/tokenizer/branch/master/graph/badge.svg?token=MFY5NWATGC)](https://codecov.io/gh/bzick/tokenizer)
[![Go Report Card](https://goreportcard.com/badge/github.com/bzick/tokenizer?rnd=2)](https://goreportcard.com/report/github.com/bzick/tokenizer)
[![GoDoc](https://godoc.org/github.com/bzick/tokenizer?status.svg)](https://godoc.org/github.com/bzick/tokenizer)

High-performance, generic tokenizer (lexer) for Go.
It parses any string, byte slice or infinite `io.Reader` stream into a stream of typed tokens.
Use it as the foundation for higher-level parsers and DSLs.

## Features

- **Fast.** Single pass over the data, zero allocations per token on the hot path (token values point directly into the source buffer — no copies), lookup tables instead of regexp.
- **Simple [API](https://pkg.go.dev/github.com/bzick/tokenizer).**
- Recognizes [integer](#integer-number) and [float](#float-number) numbers.
- Recognizes [quoted (framed) strings](#framed-string) with [escaping](#framed-string) and [embedded injections](#injections-in-a-framed-string).
- [User-defined tokens](#user-defined-tokens) (operators, punctuation, keywords).
- Unicode-aware keywords.
- Configurable whitespace symbols.
- Streams infinite input without loading it fully into memory and without panicking.

## Use cases

- Parsing text formats: XML, HTML, [JSON](./example_test.go), YAML, etc.
- Parsing huge or infinite inputs.
- Parsing programming languages, DSLs, templates and formulas.

## Installation

```bash
go get github.com/bzick/tokenizer
```

## Table of contents

- [Quick start](#quick-start)
- [Getting started](#getting-started)
  - [Create and parse](#create-and-parse)
  - [Parse an infinite stream](#parse-an-infinite-stream)
- [Built-in tokens](#built-in-tokens)
  - [Unknown token](#unknown-token)
  - [Keywords](#keywords)
  - [Integer number](#integer-number)
  - [Float number](#float-number)
  - [Framed string](#framed-string)
  - [Injections in a framed string](#injections-in-a-framed-string)
- [User-defined tokens](#user-defined-tokens)
- [Stream API](#stream-api)
- [Known issues](#known-issues)
- [Benchmark](#benchmark)

## Quick start

For example, parsing the SQL `WHERE` condition `user_id = 119 and modified > "2020-01-01 00:00:00" or amount >= 122.34`:

```go
import "github.com/bzick/tokenizer"

// define keys for the custom tokens
const (
	TEquality = iota + 1
	TDot
	TMath
	TDoubleQuoted
)

// configure the tokenizer
parser := tokenizer.New()
parser.DefineTokens(TEquality, []string{"<", "<=", "==", ">=", ">", "!="})
parser.DefineTokens(TDot, []string{"."})
parser.DefineTokens(TMath, []string{"+", "-", "/", "*", "%"})
parser.DefineStringToken(TDoubleQuoted, `"`, `"`).SetEscapeSymbol(tokenizer.BackSlash)
parser.AllowKeywordSymbols(tokenizer.Underscore, tokenizer.Numbers)

// create the token stream
stream := parser.ParseString(`user_id = 119 and modified > "2020-01-01 00:00:00" or amount >= 122.34`)
defer stream.Close()

// iterate over the tokens
for stream.IsValid() {
	if stream.CurrentToken().Is(tokenizer.TokenKeyword) {
		field := stream.CurrentToken().ValueString()
		// ...
		_ = field
	}
	stream.GoNext()
}
```

The stream of tokens:

```
string:  user_id  =  119  and  modified  >  "2020-01-01 00:00:00"  or  amount  >=  122.34
tokens: |user_id| =| 119| and| modified| >| "2020-01-01 00:00:00"| or| amount| >=| 122.34|
        |   0   | 1|  2 |  3 |    4     | 5|          6           | 7|    8  | 9 |   10  |

 0: {key: TokenKeyword, value: "user_id"}                 token.ValueString()          == "user_id"
 1: {key: TEquality,    value: "="}                       token.ValueString()          == "="
 2: {key: TokenInteger, value: "119"}                     token.ValueInt64()           == 119
 3: {key: TokenKeyword, value: "and"}                     token.ValueString()          == "and"
 4: {key: TokenKeyword, value: "modified"}                token.ValueString()          == "modified"
 5: {key: TEquality,    value: ">"}                       token.ValueString()          == ">"
 6: {key: TokenString,  value: "\"2020-01-01 00:00:00\""} token.ValueUnescapedString() == "2020-01-01 00:00:00"
 7: {key: TokenKeyword, value: "or"}                      token.ValueString()          == "or"
 8: {key: TokenKeyword, value: "amount"}                  token.ValueString()          == "amount"
 9: {key: TEquality,    value: ">="}                      token.ValueString()          == ">="
10: {key: TokenFloat,   value: "122.34"}                  token.ValueFloat64()         == 122.34
```

More examples:

- [JSON parser](./example_test.go)

## Getting started

### Create and parse

```go
import "github.com/bzick/tokenizer"

parser := tokenizer.New()
parser.AllowKeywordSymbols(tokenizer.Underscore, tokenizer.Numbers)
// ... and any other configuration
```

There are two ways to **parse a string or a byte slice**. Both return a `*Stream`:

- `parser.ParseString(str)`
- `parser.ParseBytes(slice)`

Always `Close()` the stream when you are done — it releases token objects back to the internal pool:

```go
stream := parser.ParseString(`user_id = 119`)
defer stream.Close()
```

### Parse an infinite stream

The tokenizer can also parse an **endless stream** of data.
Pass an `io.Reader` and a buffer size (in bytes); data is read and parsed chunk by chunk, so the whole input never needs to fit in memory:

```go
fp, err := os.Open("data.json") // huge JSON file
// handle err, configure the tokenizer ...

stream := parser.ParseStream(fp, 4096).SetHistorySize(10)
defer stream.Close()
for stream.IsValid() {
	// ...
	stream.GoNext()
}
```

In stream mode only a window of tokens is kept in memory.
`SetHistorySize(n)` controls how many already-visited tokens remain available behind the current one (for `GoPrev`, `GoTo`, `GetSnippet`, etc.).

## Built-in tokens

Every token carries one of these built-in keys unless it matches a user-defined token:

- `tokenizer.TokenUnknown` — a symbol that does not match any known token.
- `tokenizer.TokenKeyword` — a word: any combination of letters, including unicode letters.
- `tokenizer.TokenInteger` — an integer number.
- `tokenizer.TokenFloat` — a float/double number.
- `tokenizer.TokenString` — a quoted (framed) string.
- `tokenizer.TokenStringFragment` — a fragment of a framed string that contains injections.
- `tokenizer.TokenUndef` — returned when the stream pointer is out of range (see [Stream API](#stream-api)).

### Unknown token

A token is marked as `tokenizer.TokenUnknown` when the parser encounters a symbol that does not match any known token:

```go
stream := parser.ParseString(`one!`)
```

```
stream: [
    {Key: tokenizer.TokenKeyword, Value: "one"},
    {Key: tokenizer.TokenUnknown, Value: "!"},
]
```

By default, `TokenUnknown` tokens are added to the stream.
Calling `parser.StopOnUndefinedToken()` makes the parser stop as soon as an unknown token appears:

```go
parser.StopOnUndefinedToken()
stream := parser.ParseString(`one!`)
```

```
stream: [
    {Key: tokenizer.TokenKeyword, Value: "one"},
]
```

Note that with `StopOnUndefinedToken()` enabled the input may not be fully parsed.
To detect that, compare `stream.GetParsedLength()` with the length of the original input.

### Keywords

Any word that is not a custom token is stored as a single `tokenizer.TokenKeyword`.
A keyword can contain unicode characters, and can be configured to contain additional symbols such as numbers and underscores (see `AllowKeywordSymbols`).

```go
stream := parser.ParseString(`one 二 три`)
```

```
stream: [
    {Key: tokenizer.TokenKeyword, Value: "one"},
    {Key: tokenizer.TokenKeyword, Value: "二"},
    {Key: tokenizer.TokenKeyword, Value: "три"},
]
```

Keywords can be extended with `parser.AllowKeywordSymbols(majorSymbols, minorSymbols)`:

- **Major** symbols may appear anywhere in the keyword — at the beginning, in the middle and at the end.
- **Minor** symbols may appear only in the middle and at the end (not at the beginning).

```go
parser.AllowKeywordSymbols(tokenizer.Underscore, tokenizer.Numbers)
// allows: "_one23", "__one2__two3"

parser.AllowKeywordSymbols([]rune{'_', '@'}, tokenizer.Numbers)
// allows: "one@23", "@_one_two23", "_one23", "_one2_two3", "@@one___two@_9"
```

### Integer number

Any integer is stored as one token with the key `tokenizer.TokenInteger`:

```go
stream := parser.ParseString(`223 999`)
```

```
stream: [
    {Key: tokenizer.TokenInteger, Value: "223"},
    {Key: tokenizer.TokenInteger, Value: "999"},
]
```

Use `token.ValueInt64()` to get the value as `int64`:

```go
stream := parser.ParseString("123")
fmt.Printf("Token is %d", stream.CurrentToken().ValueInt64()) // Token is 123
```

Underscores between digits (e.g. `1_000`) can be allowed with `parser.AllowNumberUnderscore()`.

### Float number

Any float is stored as one token with the key `tokenizer.TokenFloat`. A float may

- have a fractional point, e.g. `1.2`
- have an exponent, e.g. `1e6`
- use lower- or upper-case `e`/`E` in the exponent, e.g. `1E6`, `1e6`
- have a sign in the exponent, e.g. `1e-6`, `1e+6`

```go
stream := parser.ParseString(`1.3e-8`)
```

```
stream: [
    {Key: tokenizer.TokenFloat, Value: "1.3e-8"},
]
```

Use `token.ValueFloat64()` to get the value as `float64`:

```go
stream := parser.ParseString("1.3e2")
fmt.Printf("Token is %g", stream.CurrentToken().ValueFloat64()) // Token is 130
```

### Framed string

A string that is enclosed between two tokens is called a *framed string*.
The most common example is a quoted string such as `"one two"`, where the quotes are the frame (edge) tokens.

Define a framed string with `parser.DefineStringToken(key, startToken, endToken)`.
`SetEscapeSymbol` makes the escape character (usually a backslash) ignore the closing frame, and `AddSpecialStrings` lists the sequences that the escape character can unescape:

```go
const TokenDoubleQuoted = 10
// ...
parser.DefineStringToken(TokenDoubleQuoted, `"`, `"`).
	SetEscapeSymbol(tokenizer.BackSlash).
	AddSpecialStrings([]string{`"`})

stream := parser.ParseString(`"two \"three"`)
```

```
stream: [
    {Key: tokenizer.TokenString, Value: `"two \"three"`},
]
```

The raw value (`token.Value()` / `token.ValueString()`) includes the frame tokens and escape symbols.
Use `token.ValueUnescapedString()` (or `token.ValueUnescaped()` for `[]byte`) to get the string without the frame tokens and with escape sequences resolved:

```go
value := stream.CurrentToken().ValueUnescapedString() // two "three
```

`token.StringKey()` returns the key that was passed to `DefineStringToken`:

```go
if stream.CurrentToken().StringKey() == TokenDoubleQuoted {
	// ...
}
```

### Injections in a framed string

A framed string can contain embedded expressions (injections) that are parsed as regular tokens, for example `"one {{ two }} three"`.
The pieces of the string before, between and after the injections are emitted as `tokenizer.TokenStringFragment` tokens (they keep the frame quote and the surrounding whitespace); the injected part is parsed with the full set of tokens.

```go
const (
	TokenOpenInjection  = 1
	TokenCloseInjection = 2
	TokenQuotedString   = 3
)

parser := tokenizer.New()
parser.DefineTokens(TokenOpenInjection, []string{"{{"})
parser.DefineTokens(TokenCloseInjection, []string{"}}"})
parser.DefineStringToken(TokenQuotedString, `"`, `"`).
	AddInjection(TokenOpenInjection, TokenCloseInjection)

stream := parser.ParseString(`"one {{ two }} three"`)
```

```
stream: [
    {Key: tokenizer.TokenStringFragment, Value: `"one `},
    {Key: TokenOpenInjection,            Value: "{{"},
    {Key: tokenizer.TokenKeyword,        Value: "two"},
    {Key: TokenCloseInjection,           Value: "}}"},
    {Key: tokenizer.TokenStringFragment, Value: ` three"`},
]
```

Use cases: parsing templates and placeholders.

## User-defined tokens

Custom tokens (operators, punctuation, brackets, ...) are registered with `parser.DefineTokens(key, tokens)`.
The `key` must be a positive integer; a single key can map to multiple token strings.
When several tokens can match at the same position, the longest one wins (e.g. `>=` is preferred over `>`).

```go
const (
	TokenCurlyOpen TokenKey = iota + 1
	TokenCurlyClose
	TokenSquareOpen
	TokenSquareClose
	TokenColon
	TokenComma
	TokenDoubleQuoted
)

// a minimal JSON tokenizer
parser := tokenizer.New()
parser.
	DefineTokens(TokenCurlyOpen, []string{"{"}).
	DefineTokens(TokenCurlyClose, []string{"}"}).
	DefineTokens(TokenSquareOpen, []string{"["}).
	DefineTokens(TokenSquareClose, []string{"]"}).
	DefineTokens(TokenColon, []string{":"}).
	DefineTokens(TokenComma, []string{","}).
	DefineStringToken(TokenDoubleQuoted, `"`, `"`).
	SetEscapeSymbol(tokenizer.BackSlash).
	AddSpecialStrings(tokenizer.DefaultSpecialString)

stream := parser.ParseString(`{"key": [1]}`)
```

See [`example_test.go`](./example_test.go) for a complete JSON parser built on top of this configuration.

## Stream API

The `*Stream` is a bidirectional iterator over the parsed tokens. Common methods:

| Method | Description |
| --- | --- |
| `IsValid() bool` | `true` while the pointer is on a real token. |
| `GoNext() *Stream` | Move to the next token (loads the next chunk in stream mode). |
| `GoPrev() *Stream` | Move to the previous token (bounded by `SetHistorySize`). |
| `GoTo(id int) *Stream` | Move to a token by its id. |
| `CurrentToken() *Token` | The current token (a `TokenUndef` token when out of range). |
| `NextToken() / PrevToken() *Token` | Peek without moving the pointer. |
| `GoNextIfNextIs(key, ...) bool` | Advance only if the next token matches. |
| `IsNextSequence(keys...) bool` | Check that the following tokens match a sequence. |
| `GetSnippet(before, after int) []Token` | Tokens around the current one. |
| `GetParsedLength() int` | Number of bytes parsed so far. |
| `Close()` | Release tokens back to the pool. |

Token accessors:

| Method | Description |
| --- | --- |
| `Key() TokenKey` | The token key. |
| `Is(key, ...keys) bool` | Whether the token matches any of the keys. |
| `Value() []byte` / `ValueString() string` | The raw value from the source. |
| `ValueInt64() int64` / `ValueFloat64() float64` | The numeric value. |
| `ValueUnescaped() []byte` / `ValueUnescapedString() string` | A framed string without frame tokens and escapes. |
| `StringKey() TokenKey` | The key defined in `DefineStringToken`. |
| `Line() int` / `Offset() int` | Position in the input (line starts at 1). |
| `Indent() []byte` | Whitespace before the token. |

> Do not store a `*Token` returned by the stream — the underlying object may be reused as the pointer moves.
> Copy the value if you need to keep it.

## Known issues

- A zero byte (`\x00`) in the input stops parsing.

## Benchmark

Parse a string / byte slice:

```
pkg: tokenizer
cpu: Intel(R) Core(TM) i7-7820HQ CPU @ 2.90GHz
BenchmarkParseBytes
    stream_test.go:251: Speed: 70 bytes string with 19.689µs: 3555284 byte/sec
    stream_test.go:251: Speed: 7000 bytes string with 848.163µs: 8253130 byte/sec
    stream_test.go:251: Speed: 700000 bytes string with 75.685945ms: 9248744 byte/sec
    stream_test.go:251: Speed: 11093670 bytes string with 1.16611538s: 9513355 byte/sec
BenchmarkParseBytes-8   	  158481	      7358 ns/op
```

Parse an infinite stream:

```
pkg: tokenizer
cpu: Intel(R) Core(TM) i7-7820HQ CPU @ 2.90GHz
BenchmarkParseInfStream
    stream_test.go:226: Speed: 70 bytes at 33.826µs: 2069414 byte/sec
    stream_test.go:226: Speed: 7000 bytes at 627.357µs: 11157921 byte/sec
    stream_test.go:226: Speed: 700000 bytes at 27.675799ms: 25292856 byte/sec
    stream_test.go:226: Speed: 30316440 bytes at 1.18061702s: 25678471 byte/sec
BenchmarkParseInfStream-8   	  433092	      2726 ns/op
PASS
```
