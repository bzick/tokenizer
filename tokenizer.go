// Package tokenizer provides a high performance generic tokenizer (lexer)
// that can parse any string, slice or infinite buffer to any tokens.
// It is highly customizable and can be used, for example, by higher level
// parsers for writing DSLs.
package tokenizer

import (
	"io"
	"sort"
	"sync"
)

const newLine = '\n'

// TokenKey token type identifier
type TokenKey int

const (
	// TokenUnknown means that this token not embedded token and not user defined.
	TokenUnknown TokenKey = -6
	// TokenStringFragment means that this is only fragment of the quoted string with injections.
	// For example, "one {{ two }} three", where "one " and " three" — TokenStringFragment
	TokenStringFragment TokenKey = -5
	// TokenString means that this token is quoted string.
	// For example, "one two"
	TokenString TokenKey = -4
	// TokenFloat means that this token is a float number with point and/or exponent.
	// For example, 1.2, 1e6, 1E-6
	TokenFloat TokenKey = -3
	// TokenInteger means that this token is an integer number.
	// For example, 3, 49983
	TokenInteger TokenKey = -2
	// TokenKeyword means that this token is word.
	// For example, one, two, три
	TokenKeyword TokenKey = -1
	// TokenUndef means that token doesn't exist.
	// Then stream out of range of a token list any getter or checker will return TokenUndef token.
	TokenUndef TokenKey = 0
)

// BackSlash just backslash byte
const BackSlash = '\\'

var DefaultWhiteSpaces = []byte{' ', '\t', '\n', '\r'}

// DefaultStringEscapes is default escaped symbols. Those symbols are often used everywhere.
//
// Deprecated: use DefaultSpecialString and AddSpecialStrings
var DefaultStringEscapes = map[byte]byte{
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'\\': '\\',
}

// DefaultSpecialString is default escaped symbols.
var DefaultSpecialString = []string{
	"\\",
	"n",
	"r",
	"t",
}

var Numbers = []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
var Underscore = []rune{'_'}

// tokenItem describes one token.
type tokenRef struct {
	// Token type. Not unique.
	Key TokenKey
	// Token value as is. Should be unique.
	Token []byte
	// Token Alone flag, with true mean token only split with Non-continuous string
	Alone bool
	// ignore token case
	IgnoreCase bool
}

// QuoteInjectSettings describes open injection token and close injection token.
type QuoteInjectSettings struct {
	// Token type witch opens quoted string.
	StartKey TokenKey
	// Token type witch closes quoted string.
	EndKey TokenKey
}

// StringSettings describes framed(quoted) string tokens like quoted strings.
type StringSettings struct {
	Key          TokenKey
	StartToken   []byte
	EndToken     []byte
	EscapeSymbol byte
	SpecSymbols  [][]byte
	Injects      []QuoteInjectSettings
}

// AddInjection configure injection in to string.
// Injection - parsable fragment of framed(quoted) string.
// Often used for parsing of placeholders or template expressions in the framed string.
func (q *StringSettings) AddInjection(startTokenKey, endTokenKey TokenKey) *StringSettings {
	q.Injects = append(q.Injects, QuoteInjectSettings{StartKey: startTokenKey, EndKey: endTokenKey})
	return q
}

// SetEscapeSymbol set escape symbol for framed(quoted) string.
// Escape symbol allows ignoring close token of framed string.
// Also, escape symbol allows using special symbols in the frame strings, like \n, \t.
func (q *StringSettings) SetEscapeSymbol(symbol byte) *StringSettings {
	q.EscapeSymbol = symbol
	return q
}

// SetSpecialSymbols set mapping of all escapable symbols for escape symbol, like \n, \t, \r.
//
// Deprecated: use AddSpecialStrings
func (q *StringSettings) SetSpecialSymbols(special map[byte]byte) *StringSettings {
	for _, v := range special {
		q.SpecSymbols = append(q.SpecSymbols, []byte{v})
	}
	return q
}

// AddSpecialStrings set mapping of all escapable strings for escape symbol, like \n, \t, \r.
func (q *StringSettings) AddSpecialStrings(special []string) *StringSettings {
	for _, s := range special {
		q.SpecSymbols = append(q.SpecSymbols, []byte(s))
	}
	return q
}

// Tokenizer stores all token configuration and behaviors.
type Tokenizer struct {
	stopOnUnknown         bool
	allowNumberUnderscore bool
	// all defined custom tokens {key: [token1, token2, ...], ...}
	tokens map[TokenKey][]*tokenRef
	index  map[byte][]*tokenRef
	// with ignore case token index
	icIndex        map[byte][]*tokenRef
	quotes         []*StringSettings
	wSpaces        []byte
	kwMajorSymbols []rune
	kwMinorSymbols []rune
	pool           sync.Pool
}

// New creates new tokenizer.
func New() *Tokenizer {
	t := Tokenizer{
		// flags:   0,
		tokens:  map[TokenKey][]*tokenRef{},
		index:   map[byte][]*tokenRef{},
		icIndex: map[byte][]*tokenRef{},
		quotes:  []*StringSettings{},
		wSpaces: DefaultWhiteSpaces,
	}
	t.pool.New = func() interface{} {
		return new(Token)
	}
	return &t
}

// SetWhiteSpaces sets custom whitespace symbols between tokens.
// By default: `{' ', '\t', '\n', '\r'}`
func (t *Tokenizer) SetWhiteSpaces(ws []byte) *Tokenizer {
	t.wSpaces = ws
	return t
}

// AllowKeywordSymbols sets major and minor symbols for keywords.
// Major symbols (any quantity) might be in the beginning, at the middle and at the end of keyword.
// Minor symbols (any quantity) might be at the middle and at the end of the keyword.
//
//	parser.AllowKeywordSymbols(tokenizer.Underscore, tokenizer.Numbers)
//	// allows: "_one23", "__one2__two3"
//	parser.AllowKeywordSymbols([]rune{'_', '@'}, tokenizer.Numbers)
//	// allows: "one@23", "@_one_two23", "_one23", "_one2_two3", "@@one___two@_9"
//
// Beware, the tokenizer does not control consecutive duplicates of these runes.
func (t *Tokenizer) AllowKeywordSymbols(majorSymbols []rune, minorSymbols []rune) *Tokenizer {
	t.kwMajorSymbols = majorSymbols
	t.kwMinorSymbols = minorSymbols
	return t
}

// AllowKeywordUnderscore allows underscore symbol in keywords, like `one_two` or `_three`
//
// Deprecated: use AllowKeywordSymbols
func (t *Tokenizer) AllowKeywordUnderscore() *Tokenizer {
	t.kwMajorSymbols = append(t.kwMajorSymbols, '_')
	return t
}

// AllowNumbersInKeyword allows numbers in keywords, like `one1` or `r2d2`
// The method allows numbers in keywords, but the keyword itself must not start with a number.
// There should be no spaces between letters and numbers.
//
// Deprecated: use AllowKeywordSymbols
func (t *Tokenizer) AllowNumbersInKeyword() *Tokenizer {
	t.kwMinorSymbols = append(t.kwMinorSymbols, Numbers...)
	return t
}

// StopOnUndefinedToken stops parsing if unknown token detected.
func (t *Tokenizer) StopOnUndefinedToken() *Tokenizer {
	t.stopOnUnknown = true
	return t
}

// AllowNumberUnderscore allows underscore symbol in numbers, like `1_000`
func (t *Tokenizer) AllowNumberUnderscore() *Tokenizer {
	t.allowNumberUnderscore = true
	return t
}

type DefineTokenOption func(*tokenRef)

func AloneTokenOption(ref *tokenRef) {
	ref.Alone = true
}

func IgnoreCaseTokenOption(ref *tokenRef) {
	ref.IgnoreCase = true
}

// DefineTokens add custom token.
// The `key` is the identifier of `tokens`, `tokens` — slice of tokens as string.
// If a key already exists, tokens will be rewritten.
func (t *Tokenizer) DefineTokens(key TokenKey, tokens []string, options ...DefineTokenOption) *Tokenizer {
	var tks []*tokenRef
	if key < 1 {
		return t
	}
	for _, token := range tokens {
		ref := tokenRef{
			Key:        key,
			Token:      s2b(token),
			Alone:      false,
			IgnoreCase: false,
		}
		if len(options) > 0 {
			for _, option := range options {
				option(&ref)
			}
		}

		tks = append(tks, &ref)
		var index map[byte][]*tokenRef
		var head byte

		if ref.IgnoreCase {
			index = t.icIndex
			head = upperCaseAlphabet(ref.Token[0])
		} else {
			index = t.index
			head = ref.Token[0]
		}
		if index[head] == nil {
			index[head] = []*tokenRef{}
		}
		index[head] = append(index[head], &ref)
		sort.Slice(index[head], func(i, j int) bool {
			return len(index[head][i].Token) > len(index[head][j].Token)
		})
	}
	t.tokens[key] = tks

	return t
}

// DefineStringToken defines a token string.
// For example, a piece of data surrounded by quotes: "string in quotes" or 'string on single quotes'.
// Arguments startToken and endToken defines open and close "quotes".
//
//   - `t.DefineStringToken(10, "`", "`")` - parse string "one `two three`" will be parsed as
//     [{key: TokenKeyword, value: "one"}, {key: TokenString, value: "`two three`"}]
//
//   - `t.DefineStringToken(11, "//", "\n")` - parse string "parse // like comment\n" will be parsed as
//     [{key: TokenKeyword, value: "parse"}, {key: TokenString, value: "// like comment"}]
func (t *Tokenizer) DefineStringToken(key TokenKey, startToken, endToken string) *StringSettings {
	q := &StringSettings{
		Key:        key,
		StartToken: s2b(startToken),
		EndToken:   s2b(endToken),
	}
	if q.StartToken == nil {
		return q
	}
	t.quotes = append(t.quotes, q)

	return q
}

func (t *Tokenizer) allocToken() *Token {
	return t.pool.Get().(*Token)
}

func (t *Tokenizer) freeToken(token *Token) {
	token.next = nil
	token.prev = nil
	token.value = nil
	token.indent = nil
	token.offset = 0
	token.line = 0
	token.id = 0
	token.key = 0
	token.string = nil
	t.pool.Put(token)
}

// ParseString parse string into stream of tokens.
func (t *Tokenizer) ParseString(str string) *Stream {
	return t.ParseBytes(s2b(str))
}

// ParseBytes parse and convert slice of bytes into stream of tokens.
func (t *Tokenizer) ParseBytes(str []byte) *Stream {
	p := newParser(t, str)
	p.parse()
	return NewStream(p)
}

// ParseStream parse and convert infinite stream of bytes into infinite stream of tokens.
func (t *Tokenizer) ParseStream(r io.Reader, bufferSize uint) *Stream {
	p := newInfParser(t, r, bufferSize)
	p.preload()
	p.parse()
	return NewInfStream(p)
}
