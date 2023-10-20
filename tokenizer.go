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
	// TokenStringFragment means that this is only fragment of quoted string with injections
	// For example, "one {{ two }} three", where "one " and " three" — TokenStringFragment
	TokenStringFragment TokenKey = -5
	// TokenString means than this token is quoted string.
	// For example, "one two"
	TokenString TokenKey = -4
	// TokenFloat means that this token is float number with point and/or exponent.
	// For example, 1.2, 1e6, 1E-6
	TokenFloat TokenKey = -3
	// TokenInteger means that this token is integer number.
	// For example, 3, 49983
	TokenInteger TokenKey = -2
	// TokenKeyword means that this token is word.
	// For example, one, two, три
	TokenKeyword TokenKey = -1
	// TokenUndef means that token doesn't exist.
	// Then stream out of range of token list any getter or checker will return TokenUndef token.
	TokenUndef TokenKey = 0
)

const (
	fStopOnUnknown          uint16 = 0b1
	fAllowKeywordUnderscore uint16 = 0b10
	fAllowNumberUnderscore  uint16 = 0b100
	fAllowNumberInKeyword   uint16 = 0b1000
	fAllowAtInKeyword       uint16 = 0b10000
	fAllowDotInKeyword      uint16 = 0b100000
)

// BackSlash just backslash byte
const BackSlash = '\\'

var defaultWhiteSpaces = []byte{' ', '\t', '\n', '\r'}

// DefaultStringEscapes is default escaped symbols. Those symbols are often used everywhere.
var DefaultStringEscapes = map[byte]byte{
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'\\': '\\',
}

// tokenItem describes one token.
type tokenRef struct {
	// Token type. Not unique.
	Key TokenKey
	// Token value as is. Should be unique.
	Token []byte
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
	SpecSymbols  map[byte]byte
	Injects      []QuoteInjectSettings
}

// AddInjection configure injection in to string.
// Injection - parsable fragment of framed(quoted) string.
// Often used for parsing of placeholders or template's expressions in the framed string.
func (q *StringSettings) AddInjection(startTokenKey, endTokenKey TokenKey) *StringSettings {
	q.Injects = append(q.Injects, QuoteInjectSettings{StartKey: startTokenKey, EndKey: endTokenKey})
	return q
}

// SetEscapeSymbol set escape symbol for framed(quoted) string.
// Escape symbol allows ignoring close token of framed string.
// Also escape symbol allows using special symbols in the frame strings, like \n, \t.
func (q *StringSettings) SetEscapeSymbol(symbol byte) *StringSettings {
	q.EscapeSymbol = symbol
	return q
}

// SetSpecialSymbols set mapping of all escapable symbols for escape symbol, like \n, \t, \r.
func (q *StringSettings) SetSpecialSymbols(special map[byte]byte) *StringSettings {
	q.SpecSymbols = special
	return q
}

// Tokenizer stores all tokens configuration and behaviors.
type Tokenizer struct {
	// bit flags
	flags uint16
	// all defined custom tokens {key: [token1, token2, ...], ...}
	tokens  map[TokenKey][]*tokenRef
	index   map[byte][]*tokenRef
	quotes  []*StringSettings
	wSpaces []byte
	pool    sync.Pool
}

// New creates new tokenizer.
func New() *Tokenizer {
	t := Tokenizer{
		flags:   0,
		tokens:  map[TokenKey][]*tokenRef{},
		index:   map[byte][]*tokenRef{},
		quotes:  []*StringSettings{},
		wSpaces: defaultWhiteSpaces,
	}
	t.pool.New = func() interface{} {
		return new(Token)
	}
	return &t
}

// SetWhiteSpaces sets custom whitespace symbols between tokens.
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

// AllowKeywordUnderscore allows underscore symbol in keywords, like `one_two` or `_three`
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

// AllowAtInKeyword allows symbol '@' in keywords
// The method allows ats in keywords, but the keyword itself must not start with an at.
// There should be no spaces between letters and ats.
func (t *Tokenizer) AllowAtInKeyword() *Tokenizer {
	t.flags |= fAllowAtInKeyword
	return t
}

// AllowDotInKeyword allows symbol '.' in keywords
// The method allows dots in keywords, but the keyword itself must not start with a dot.
// There should be no spaces between letters and dots.
func (t *Tokenizer) AllowDotInKeyword() *Tokenizer {
	t.flags |= fAllowDotInKeyword
	return t
}

// DefineTokens add custom token.
// There `key` unique is identifier of `tokens`, `tokens` — slice of string of tokens.
// If key already exists tokens will be rewritten.
func (t *Tokenizer) DefineTokens(key TokenKey, tokens []string) *Tokenizer {
	var tks []*tokenRef
	if key < 1 {
		return t
	}
	for _, token := range tokens {
		ref := tokenRef{
			Key:   key,
			Token: s2b(token),
		}
		head := ref.Token[0]
		tks = append(tks, &ref)
		if t.index[head] == nil {
			t.index[head] = []*tokenRef{}
		}
		t.index[head] = append(t.index[head], &ref)
		sort.Slice(t.index[head], func(i, j int) bool {
			return len(t.index[head][i].Token) > len(t.index[head][j].Token)
		})
	}
	t.tokens[key] = tks

	return t
}

// DefineStringToken defines a token string.
// For example, a piece of data surrounded by quotes: "string in quotes" or 'string on sigle quotes'.
// Arguments startToken and endToken defines open and close "quotes".
//   - t.DefineStringToken("`", "`") - parse string "one `two three`" will be parsed as
//     [{key: TokenKeyword, value: "one"}, {key: TokenString, value: "`two three`"}]
//   - t.DefineStringToken("//", "\n") - parse string "parse // like comment\n" will be parsed as
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

// ParseStream parse the string into tokens.
func (t *Tokenizer) ParseStream(r io.Reader, bufferSize uint) *Stream {
	p := newInfParser(t, r, bufferSize)
	p.preload()
	p.parse()
	return NewInfStream(p)
}
