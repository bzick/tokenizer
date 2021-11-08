package tokenizer

import (
	"io"
	"sync"
)

const newLine = '\n'

const (
	// TokenUnknown means that this token not embedded token and not user defined.
	TokenUnknown = -6
	// TokenStringFragment means that this is only fragment of quoted string with injections
	// For example, "one {{ two }} three", where "one " and " three" — TokenStringFragment
	TokenStringFragment = -5
	// TokenString means than this token is quoted string.
	// For example, "one two"
	TokenString = -4
	// TokenFloat means that this token is float number with point and/or exponent.
	// For example, 1.2, 1e6, 1E-6
	TokenFloat = -3
	// TokenInteger means that this token is integer number.
	// For example, 3, 49983
	TokenInteger = -2
	// TokenKeyword means that this token is word.
	// For example, one, two, три
	TokenKeyword = -1
	// TokenUndef means that token doesn't exist.
	// Then stream out of range of token list any getter or checker will return TokenUndef token.
	TokenUndef = 0
)

const (
	fStopOnUnknown          = 0b1
	fAllowKeywordUnderscore = 0b10
	fAllowNumberUnderscore  = 0b100
	fAllowNumberInKeyword   = 0b1000
)

var defaultWhiteSpaces = []byte{' ', '\t', '\n', '\r'}
var DefaultStringEscapes = map[string]byte{
	"n":  '\n',
	"r":  '\r',
	"t":  '\t',
	"\\": '\\',
}

// TokenSettings describes one token.
type TokenSettings struct {
	// Token type. Not unique.
	Key int
	// Token value as is. Should be unique.
	Token []byte
}

// QuoteInjectSettings describes open injection token and close injection token.
type QuoteInjectSettings struct {
	// Token type witch opens quoted string.
	StartKey int
	// Token type witch closes quoted string.
	EndKey int
}

// QuoteSettings describes quoted string tokens.
type QuoteSettings struct {
	StartToken   []byte
	EndToken     []byte
	EscapeSymbol byte
	SpecSymbols  map[string]byte
	Injects      []QuoteInjectSettings
}

// AddInjection configure injection in segment
func (q *QuoteSettings) AddInjection(startTokenKey, endTokenKey int) *QuoteSettings {
	q.Injects = append(q.Injects, QuoteInjectSettings{StartKey: startTokenKey, EndKey: endTokenKey})
	return q
}

func (q *QuoteSettings) SetEscapeSymbol(symbol byte) *QuoteSettings {
	q.EscapeSymbol = symbol
	return q
}

func (q *QuoteSettings) SetSpecialSymbols(special map[string]byte) *QuoteSettings {
	q.SpecSymbols = special
	return q
}

type Tokenizer struct {
	// bit flags
	flags uint16
	// all defined custom tokens
	tokens []TokenSettings
	//
	tokensMap map[int][]*TokenSettings
	quotes    []*QuoteSettings
	wSpaces   []byte

	pool sync.Pool
}

// New creates new tokenizer.
func New() *Tokenizer {
	t := Tokenizer{
		flags:     0,
		tokens:    []TokenSettings{},
		tokensMap: map[int][]*TokenSettings{},
		quotes:    []*QuoteSettings{},
		wSpaces:   defaultWhiteSpaces,
	}
	t.pool.New = func() interface{} {
		return new(Token)
	}
	return &t
}

// SetWhiteSpaces sets custom whitespace symbols
// By default: {' ', '\t', '\n', '\r'}
func (t *Tokenizer) SetWhiteSpaces(ws []byte) *Tokenizer {
	t.wSpaces = ws
	return t
}

// StopOnUndefinedToken stops parsing if unknown token detected.
func (t *Tokenizer) StopOnUndefinedToken() *Tokenizer {
	t.flags |= fStopOnUnknown
	return t
}

// AllowKeywordUnderscore allows underscore in keywords, like `one_two` or `_three`
func (t *Tokenizer) AllowKeywordUnderscore() *Tokenizer {
	t.flags |= fAllowKeywordUnderscore
	return t
}

// AllowNumbersInKeyword allows numbers in keywords, like `one1` or `r2d2`
// The method allows numbers in keywords, but the keyword itself must not start with a number.
// There should be no spaces between letters and numbers.
func (t *Tokenizer) AllowNumbersInKeyword() *Tokenizer {
	t.flags |= fAllowNumberInKeyword
	return t
}

// AddToken add custom token.
// There key is identifier of tokens, tokens slice is string representation of tokens.
func (t *Tokenizer) AddToken(key int, tokens []string) *Tokenizer {
	if t.tokensMap[key] == nil {
		t.tokensMap[key] = []*TokenSettings{}
	}
	for _, token := range tokens {
		t.tokens = append(t.tokens, TokenSettings{
			Key:   key,
			Token: s2b(token),
		})
		t.tokensMap[key] = append(t.tokensMap[key], &t.tokens[len(t.tokens)-1])
	}
	return t
}

// AddString defines a token string.
// For example, a piece of data surrounded by quotes: "string in quotes" or 'string on sigle quotes'.
// Arguments startToken and endToken defines open and close "quotes".
//  - t.AddString("`", "`") - parse string "one `two three`" will be parsed as
// 			[{key: TokenKeyword, value: "one"}, {key: TokenString, value: "`two three`"}]
//  - t.AddString("//", "\n") - parse string "parse // like comment" will be parsed as
//			[{key: TokenKeyword, value: "parse"}, {key: TokenString, value: "// like comment"}]
func (t *Tokenizer) AddString(startToken, endToken string) *QuoteSettings {
	q := &QuoteSettings{
		StartToken: s2b(startToken),
		EndToken:   s2b(endToken),
	}
	if q.StartToken == nil {
		return q
	}
	t.quotes = append(t.quotes, q)

	return q
}

func (t *Tokenizer) getToken() *Token {
	return t.pool.Get().(*Token)
}

func (t *Tokenizer) putToken(token *Token) {
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

// ParseString parse the string into tokens
func (t *Tokenizer) ParseString(str string) *Stream {
	return t.ParseBytes(s2b(str))
}

// ParseBytes parse the bytes slice into tokens
func (t *Tokenizer) ParseBytes(str []byte) *Stream {
	p := newParser(t, str)
	p.parse()
	return NewStream(p)
}

type ParseSettings struct {
	BufferSize int
}

func (t *Tokenizer) ParseStream(r io.Reader, bufferSize uint) *Stream {
	p := newInfParser(t, r, bufferSize)
	p.preload()
	return NewInfStream(p)
}
