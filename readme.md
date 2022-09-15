# Tokenizer 

[![Build Status](https://github.com/bzick/tokenizer/actions/workflows/tokenizer.yml/badge.svg)](https://github.com/bzick/tokenizer/actions/workflows/tokenizer.yml)
[![codecov](https://codecov.io/gh/bzick/tokenizer/branch/master/graph/badge.svg?token=MFY5NWATGC)](https://codecov.io/gh/bzick/tokenizer)
[![Go Report Card](https://goreportcard.com/badge/github.com/bzick/tokenizer?rnd=2)](https://goreportcard.com/report/github.com/bzick/tokenizer)
[![GoDoc](https://godoc.org/github.com/bzick/tokenizer?status.svg)](https://godoc.org/github.com/bzick/tokenizer)

Tokenizer — parse any string, slice or infinite buffer to any tokens.

Main features:

* High performance.
* No regexp.
* Provides [simple API](https://pkg.go.dev/github.com/bzick/tokenizer).
* Supports [integer](#integer-number) and [float](#float-number) numbers.
* Supports [quoted string or other "framed"](#framed-string) strings.
* Supports [injection](#injection-in-framed-string) in quoted or "framed" strings.
* Supports unicode.
* [Customization of tokens](#user-defined-tokens).
* Autodetect white space symbols.
* Parse any data syntax (xml, [json](https://github.com/bzick/tokenizer/blob/master/example_test.go), yaml), any programming language.
* Single pass through the data.
* Parses [infinite incoming data](#parse-buffer) and don't panic.

Use cases:
- Parsing html, xml, [json](./example_test.go), yaml and other text formats.
- Parsing huge or infinite texts. 
- Parsing any programming languages.
- Parsing templates.
- Parsing formulas.

For example, parsing SQL `WHERE` condition `user_id = 119 and modified > "2020-01-01 00:00:00" or amount >= 122.34`:

```go
// define custom tokens keys
const ( 
	TEquality = 1
	TDot      = 2
	TMath     = 3
)

// configure tokenizer
parser := tokenizer.New()
parser.DefineTokens(TEquality, []string{"<", "<=", "==", ">=", ">", "!="})
parser.DefineTokens(TDot, []string{"."})
parser.DefineTokens(TMath, []string{"+", "-", "/", "*", "%"})
parser.DefineStringToken(`"`, `"`).SetEscapeSymbol(tokenizer.BackSlash)

// create tokens stream
stream := parser.ParseString(`user_id = 119 and modified > "2020-01-01 00:00:00" or amount >= 122.34`)
defer stream.Close()

// iterate over each token
for stream.Valid() {
	if stream.CurrentToken().Is(tokenizer.TokenKeyword) {
		field := stream.CurrentToken().ValueString()
		// ... 
	}
	stream.Next()
}
```

tokens stram:
```
string:  user_id  =  119  and  modified  >  "2020-01-01 00:00:00"  or  amount  >=  122.34
tokens: |user_id| =| 119| and| modified| >| "2020-01-01 00:00:00"| or| amount| >=| 122.34|
        |   0   | 1|  2 |  3 |    4    | 5|            6         | 7 |    8  | 9 |    10 |

0:  {key: TokenKeyword, value: "user_id"}                token.Value()          == "user_id"
1:  {key: TEquality, value: "="}                         token.Value()          == "="
2:  {key: TokenInteger, value: "119"}                    token.ValueInt()       == 119
3:  {key: TokenKeyword, value: "and"}                    token.Value()          == "and"
4:  {key: TokenKeyword, value: "modified"}               token.Value()          == "modified"
5:  {key: TEquality, value: ">"}                         token.Value()          == ">"
6:  {key: TokenString, value: "\"2020-01-01 00:00:00\""} token.ValueUnescaped() == "2020-01-01 00:00:00"
7:  {key: TokenKeyword, value: "or"}                     token.Value()          == "and"
8:  {key: TokenKeyword, value: "amount"}                 token.Value()          == "amount"
9:  {key: TEquality, value: ">="}                        token.Value()          == ">="
10: {key: TokenFloat, value: "122.34"}                   token.ValueFloat()     == 122.34
```

More examples:
- [JSON parser](./example_test.go)

## Begin

### Create and parse

```go
import (
    "github.com/bzick/tokenizer"
)

var parser := tokenizer.New()
parser.AllowKeywordUnderscore() // ... and other configuration code

```

There is two ways to **parse string or slice**:

- `parser.ParseString(str)`
- `parser.ParseBytes(slice)`

The package allows to **parse an endless stream** of data into tokens.
For parsing, you need to pass `io.Reader`, from which data will be read (chunk-by-chunk):

```go
fp, err := os.Open("data.json") // huge JSON file
// check fs, configure tokenizer ...

stream := parser.ParseStream(fp, 4096).SetHistorySize(10)
defer stream.Close()
for stream.IsValid() { 
	// ...
	stream.Next()
}
```

## Embedded tokens

- `tokenizer.TokenUnknown` — unspecified token key. 
- `tokenizer.TokenKeyword` — keyword, any combination of letters, including unicode letters.
- `tokenizer.TokenInteger` — integer value
- `tokenizer.TokenFloat` — float/double value
- `tokenizer.TokenString` — quoted string
- `tokenizer.TokenStringFragment` — fragment framed (quoted) string 

### Unknown token — `tokenizer.TokenUnknown`

A token marks as `TokenUnknown` if the parser detects an unknown token:

```go
parser.ParseString(`one!`)
```
```
{
    {
        Key: tokenizer.TokenKeyword
        Value: "One"
    },
    {
        Key: tokenizer.TokenUnknown
        Value: "!"
    }
}
```

By default, `TokenUnknown` tokens are added to the stream. 
To exclude them from the stream, use the `tokenizer.StopOnUndefinedToken()` method

```
{
    {
        Key: tokenizer.TokenKeyword
        Value: "one"
    }
}
```

Please note that if the `tokenizer.StopOnUndefinedToken` setting is enabled, then the string may not be fully parsed.
To find out that the string was not fully parsed, check the length of the parsed string `stream.GetParsedLength()`
and the length of the original string.

### Keywords

Any word that is not a custom token is stored in a single token as `tokenizer.TokenKeyword`.

The word can contains unicode characters, numbers (see `tokenizer.AllowNumbersInKeyword ()`) and underscore (see `tokenizer.AllowKeywordUnderscore ()`).

```go
parser.ParseString(`one two четыре`)
```
```
tokens: {
    {
        Key: tokenizer.TokenKeyword
        Value: "one"
    },
    {
        Key: tokenizer.TokenKeyword
        Value: "two"
    },
    {
        Key: tokenizer.TokenKeyword
        Value: "четыре"
    }
}
```

### Integer number

Any integer is stored as one token with key `tokenizer.Token Integer`.

```go
parser.ParseString(`223 999`)
```
```
tokens: {
    {
        Key: tokenizer.TokenInteger
        Value: "223"
    },
    {
        Key: tokenizer.TokenInteger
        Value: "999"
    },
}
```

To get int64 from the token value use `stream.GetInt()`:

```go
stream := tokenizer.ParseString("123")
fmt.Print("Token is %d", stream.CurrentToken().GetInt())  // Token is 123
```

### Float number

Any float number is stored as one token with key `tokenizer.TokenFloat`. Float number may
- have point, for example `1.2`
- have exponent, for example `1e6`
- have lower `e` or upper `E` letter in the exponent, for example `1E6`, `1e6`
- have sign in the exponent, for example `1e-6`, `1e6`, `1e+6`

```
tokenizer.ParseString(`1.3e-8`):
{
    {
        Key: tokenizer.TokenFloat
        Value: "1.3e-8"
    },
}
```

To get float64 from the token value use `token.GetFloat()`:

```go
stream := tokenizer.ParseString("1.3e2")
fmt.Print("Token is %d", stream.CurrentToken().GetFloat())  // Token is 130
```

### Framed string

Strings that are framed with tokens are called framed strings. An obvious example is quoted a string like `"one two"`.
There quotes — edge tokens.

You can create and customize framed string through `tokenizer.AddQuote()`:

```go
const TokenDoubleQuotedString = 10
tokenizer.DefineStringToken(TokenDoubleQuotedString, `"`, `"`).SetEscapeSymbol('\\')

stream := tokenizer.ParseString(`"two \"three"`)
```
```
{
    {
        Key: tokenizer.TokenString
        Value: "\"two \\"three\""
    },
}
```

To get a framed string without edge tokens and special characters, use the `stream.ValueUnescape()` method:

```go
v := stream.CurrentToken().ValueUnescape() // result: two "three
```

The method `token.StringKey()` will be return token string key defined in the `DefineStringToken`:

```go
stream.CurrentToken().StringKey() == TokenDoubleQuotedString // true
```

### Injection in framed string

Strings can contain expression substitutions that can be parsed into tokens. For example `"one {{two}} three"`.
Fragments of strings before, between and after substitutions will be stored in tokens as `tokenizer.TokenStringFragment`. 

```go
const (
    TokenOpenInjection = 1
    TokenCloseInjection = 2
    TokenQuotedString = 3
)

parser := tokenizer.New()
parser.DefineTokens(TokenOpenInjection, []string{"{{"})
parser.DefineTokens(TokenCloseInjection, []string{"}}"})
parser.DefineStringToken(TokenQuotedString, `"`, `"`).AddInjection(TokenOpenInjection, TokenCloseInjection)

parser.ParseString(`"one {{ two }} three"`)
```
Tokens:
```
{
    {
        Key: tokenizer.TokenStringFragment,
        Value: "one"
    },
    {
        Key: TokenOpenInjection,
        Value: "{{"
    },
    {
        Key: tokenizer.TokenKeyword,
        Value: "two"
    },
    {
        Key: TokenCloseInjection,
        Value: "}}"
    },
    {
        Key: tokenizer.TokenStringFragment,
        Value: "three"
    },
}
```

Use cases:
- parse templates
- parse placeholders

## User defined tokens

The new token can be defined via the `DefineTokens` method:

```go

const (
    TokenCurlyOpen    = 1
    TokenCurlyClose   = 2
    TokenSquareOpen   = 3
    TokenSquareClose  = 4
    TokenColon        = 5
    TokenComma        = 6
	TokenDoubleQuoted = 7
)

// json parser
parser := tokenizer.New()
parser.
	DefineTokens(TokenCurlyOpen, []string{"{"}).
	DefineTokens(TokenCurlyClose, []string{"}"}).
	DefineTokens(TokenSquareOpen, []string{"["}).
	DefineTokens(TokenSquareClose, []string{"]"}).
	DefineTokens(TokenColon, []string{":"}).
	DefineTokens(TokenComma, []string{","}).
	DefineStringToken(TokenDoubleQuoted, `"`, `"`).SetSpecialSymbols(tokenizer.DefaultStringEscapes)

stream := parser.ParseString(`{"key": [1]}`)
```


## Known issues

* zero-byte `\0` ignores in the source string.

## Benchmark

Parse string/bytes
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

Parse infinite stream
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
