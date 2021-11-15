Tokenizer
=====

Tokenizer — parse any string, slice or infinite buffer to any tokens.

Main features:

* High performance.
* No regexp.
* Provides simple API.
* By default, support integer and float numbers.
* Support quoted string or other "framed" strings.
* Support token injection in quoted or "framed" strings.
* Support unicode.
* Customization of tokens.
* Autodetect white space symbols.
* Parse any data syntax (xml, json, yaml), any programming language.
* Single pass through the data.
* Parsing infinite data.

Use cases:
- Parsing html, xml, json, yaml and other text formats.
- Parsing any programming language.
- Parsing string templates.
- Parsing formulas.


Example:

```go

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
parser.AddString(`"`, `"`).SetEscapeSymbol(tokenizer.BackSlash)

// parse data
stream := parser.ParseString(`user_id = 119 and modified > "2020-01-01 00:00:00" or amount >= 122.34`)
for stream.Valid() {
	if stream.Is(tokenizer.TokenKeyword) {
		// ... 
	}
	stream.GoNext()
}
```

parsing details:

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

## Embedded tokens

- `tokenizer.TokenUnknown` — unspecified token key. 
- `tokenizer.TokenKeyword` — keyword, any combination of letters, including unicode letters.
- `tokenizer.TokenInteger` — integer value
- `tokenizer.TokenFloat` — float/double value
- `tokenizer.TokenString` — quoted string
- `tokenizer.TokenStringFragment` — fragment quoted string. Quoted string may be split by placeholders. 

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

The word can contain unicode characters, numbers (see `tokenizer.AllowNumbersInKeyword ()`) and underscore (see `tokenizer.AllowKeywordUnderscore ()`).

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

Any integer is stored in one token `tokenizer.Token Integer`.

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
fmt.Print("Token is %d", stream.GetInt())  // Token is 123
```

### Float number

Any float number is stored in one token `tokenizer.TokenFloat`. Float number may
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

To get float64 from the token value use `stream.GetFloat()`:

```go
stream := tokenizer.ParseString("1.3e2")
fmt.Print("Token is %d", stream.GetFloat())  // Token is 1300
```

### Frame (quoted) string

Что бы строка считалось quoted нужно через метод `tokenizer.AddQuote()` указать открывающий токен и закрывающий токен.

```go
tokenizer.AddQuote(`"`, `"`)
stream := tokenizer.ParseString(`one "two three"`)
```
```
{
    {
        Key: tokenizer.TokenKeyword
        Value: "one"
    },
    {
        Key: tokenizer.TokenQuotedString
        Value: "\"two three\""
    },
}
```

To get a string without special characters, use the `stream.ValueUnescape()` method.

### Quoted строки с подстановкой

В quoted строках можно использовать подстановки выражений, которые можно разобрать в токены. Например `"one {{ two }} three"`.
Части quoted строк до, между и после подстановок будут помещаться в токены типа `tokenizer.TokenQuotedStringFragment` 

```go
const (
    TokenOpenInjection = 1
    TokenCloseInjection = 2
)

parser := tokenizer.New()
parser.AddToken(TokenOpenInjection, []string{"{{"})
parser.AddToken(TokenCloseInjection, []string{"}}"})
parser.AddQuote(`"`, `"`).AddInjection(TokenOpenInjection, TokenCloseInjection)

parser.ParseString(`"one {{ two }} three"`)
```

Результат будет
```
{
    {
        Key: tokenizer.TokenQuotedStringFragment,
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
        Value: "{{"
    },
    {
        Key: tokenizer.TokenQuotedStringFragment,
        Value: "three"
    },
}
```

## User defined tokens

## Parsing

### Parse string

### Parse buffer

## Known issues

* zero-byte `\0` will be ignored in the source string.

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