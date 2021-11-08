Tokenizer
=====

Tokenizer — parse any string, slice or infinite buffer to any tokens stream.

Main features:

* High performance.
* Не выделяет память при разборе строки.
* No regexp.
* Provides simple API.
* Support integer and float numbers.
* Support quoted string.
* Support token injection in quoted strings.
* Support unicode.
* Customization of tokens.
* Autodetect white space symbols.
* Можно парсить любой язык программирования.
* Единичный проход всей строки данных.
* Потоковый разбор данных на токены из бесконечного буффера.

More examples:
- [JSON parser](./example_test.go)

Explain parsing:
```
string:  user_id  =  119  and  modified  >  "2020-01-01 00:00:00"  or  amount  >=  122.34
tokens: |user_id| =| 119| and| modified| >| "2020-01-01 00:00:00"| or| amount| >=| 122.34|
        |   0   | 1|  2 |  3 |    4    | 5|            6         | 7 |    8  | 9 |    10 |

0:  {key: TokenKeyword, value: "user_id"}                token.Value()          == "user_id"
1:  {key: <your token>, value: "="}                      token.Value()          == "="
2:  {key: TokenInteger, value: "119"}                    token.ValueInt()       == 119
3:  {key: TokenKeyword, value: "and"}                    token.Value()          == "and"
4:  {key: TokenKeyword, value: "modified"}               token.Value()          == "modified"
5:  {key: <your token>, value: ">"}                      token.Value()          == ">"
6:  {key: TokenString, value: "\"2020-01-01 00:00:00\""} token.ValueUnescaped() == "2020-01-01 00:00:00"
7:  {key: TokenKeyword, value: "or"}                     token.Value()          == "and"
8:  {key: TokenKeyword, value: "amount"}                 token.Value()          == "amount"
9:  {key: <your token>, value: ">="}                     token.Value()          == ">="
10: {key: TokenFloat, value: "122.34"}                   token.ValueFloat()     == 122.34
```

## Embedded tokens

- `tokenizer.TokenUnknown` — unspecified token key. 
- `tokenizer.TokenKeyword` — keyword, any combination of letters, including unicode letters.
- `tokenizer.TokenInteger` — integer value
- `tokenizer.TokenFloat` — float/double/decimal value
- `tokenizer.TokenString` — quoted string
- `tokenizer.TokenStringFragment` — fragment quoted string. Quoted string may be split by placeholders. 

### Unknown tokens

Токен помечается как `TokenUnknown` если парсер встретил не зарегистрированный токен:
```
//tokenizer.ParseString(`one!`)
tokens: {
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

По умолчанию TokenUndef токены включаются в общ и поток токенов. 
Но можно настроить токенайзер `tokenizer.StopOnUndefinedToken()` так что бы при встрече TokenUndef токена, разбор строки прекращался:

```
// tokenizer.ParseString(`one!`)
tokens: {
    {
        Key: tokenizer.TokenKeyword
        Value: "one"
    }
}
```

Учтите что если включена настройка StopOnUndefinedToken то строка может быть разобрана не до конца.
Что бы узнать что строка была разобрана не до конца проверьте длину разобранной строки `stream.GetParsedLength()` 
и длину оригинальной строки. 

### Keywords

Любая последовательность букв (слова), включая unicode, разбирается в токен типа `tokenizer.TokenKeyword`.

```
// tokenizer.ParseString(`one two четыре`)
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

Любое цело число помещается в один токен `tokenizer.TokenInteger`

```
// tokenizer.ParseString(`223 999`)
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

Что бы получить int64 от значения токена используйте `stream.GetInt()`:

```go
stream := tokenizer.ParseString("123")
fmt.Print("Token is %d", stream.GetInt())  // Token is 123
```

### Decimal number

Любое дробное число помещается в один токен `tokenizer.TokenDecimal`. Дробное число может
- иметь точку, например 1.2
- иметь степень десяти, например 1e6
- степень десяти может быть как маленькое `e` так и большое `E`, например 1E6
- иметь знак степени, например 1e-6

```
tokenizer.ParseString(`1.3e-8`):
{
    {
        Key: tokenizer.TokenDecimal
        Value: "1.3e-8"
    },
}
```

Что бы получить float64 от значения токена используйте `stream.GetFloat()`:

```go
stream := tokenizer.ParseString("1.3e2")
fmt.Print("Token is %d", stream.GetFloat())  // Token is 1300
```

### Quoted строки

Что бы строка считалось quoted нужно через метод `tokenizer.AddQuote()` указать открывающий токен и закрывающий токен.

```go
tokenizer.AddQuote(`"`, `"`)
stream := tokenizer.ParseString(`one "two three"`)
```

Результат токены будут
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

Что бы получить строку, которая quoted можно через метод `stream.GetUnquotedString()`.

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
```js
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

