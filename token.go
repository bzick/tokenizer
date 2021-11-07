package tokenizer

import (
	"fmt"
	"strconv"
)

var undefToken = &Token{}

type Token struct {
	id     int
	key    int
	value  []byte
	line   int
	offset int
	indent []byte
	string *QuoteSettings

	prev *Token
	next *Token
}

// addNext add new token as next node of dl-list and return these token.
func (t *Token) addNext(next *Token) *Token {
	next.prev = t
	t.next = next
	return next
}

// remove this token from dl-list and fix links of prev and next nodes.
// Method returns next token or nil if no next token found.
func (t *Token) remove() *Token {
	next := t.next
	t.next.prev = nil
	t.next = nil
	t.prev = nil

	return next
}

// Id returns id of token. ID is the sequence number of tokens in the stream.
func (t *Token) Id() int {
	return t.id
}

// String returns a multiline string with the token's information.
func (t Token) String() string {
	return fmt.Sprintf("{\n\tId: %d\n\tKey: %d\n\tValue: %s\n\tPosition: %d\n\tIndent: %d bytes\n\tLine: %d\n}",
		t.id, t.key, t.value, t.offset, len(t.indent), t.line)
}

// IsValid checks if this token is valid — the key is not TokenUndef.
func (t *Token) IsValid() bool {
	return t.key != TokenUndef
}

// IsKeyword checks if this is keyword — the key is TokenKeyword.
func (t Token) IsKeyword() bool {
	return t.key == TokenKeyword
}

// IsNumber checks if this token is integer or float — the key is TokenInteger or TokenFloat.
func (t Token) IsNumber() bool {
	return t.key == TokenInteger || t.key == TokenFloat
}

// IsFloat checks if this token is float — the key is TokenFloat.
func (t Token) IsFloat() bool {
	return t.key == TokenFloat
}

// IsInteger checks if this token is integer — the key is TokenInteger.
func (t Token) IsInteger() bool {
	return t.key == TokenInteger
}

// ValueInt returns value as int64.
// If the token is float the result wild be round by math's rules.
// If the token is not TokenInteger or TokenFloat zero will be returned.
func (t Token) ValueInt() int64 {
	if t.key == TokenInteger {
		num, _ := strconv.ParseInt(b2s(t.value), 10, 64)
		return num
	} else if t.key == TokenFloat {
		num, _ := strconv.ParseFloat(b2s(t.value), 64)
		return int64(num)
	}
	return 0
}

// ValueFloat returns value as float64.
// If the token is not TokenInteger or TokenFloat zero will be returned.
func (t *Token) ValueFloat() float64 {
	if t.key == TokenFloat {
		num, _ := strconv.ParseFloat(b2s(t.value), 64)
		return num
	} else if t.key == TokenInteger {
		num, _ := strconv.ParseInt(b2s(t.value), 10, 64)
		return float64(num)
	}
	return 0.0
}

// Indent returns spaces before the token.
func (t *Token) Indent() []byte {
	return t.indent
}

// Key returns the key of the token pointed to by the pointer.
// If pointer is not valid (see IsValid) TokenUndef will be returned.
func (t *Token) Key() int {
	return t.key
}

// Value returns value of current token as slice of bytes from source.
// If current token is invalid value returns nil.
func (t *Token) Value() []byte {
	return t.value
}

// ValueAsString returns value of the token as string.
// Optional maxLength specify max length of result string.
// If string greater than maxLength method removes some runes in the middle of the string.
// If the token is TokenUndef method returns empty string.
func (t *Token) ValueAsString(maxLength int) string {
	if t.value == nil {
		return ""
	} else if maxLength > 0 && (t.key == TokenString || t.key == TokenStringFragment) {
		// todo truncate the string
		return b2s(t.value)
	} else {
		return b2s(t.value)
	}
}

// Line returns number of line of token in the source text.
// Line numbers starts from 1.
func (t *Token) Line() int {
	return t.line
}

// Offset returns the position of the token in the data source of bytes from start.
func (t *Token) Offset() int {
	return t.offset
}

// IsString checks if current token is a quoted string.
// Token key may be TokenString or TokenStringFragment.
func (t Token) IsString() bool {
	return t.key == TokenString || t.key == TokenStringFragment
}

// ValueUnescapedString returns unquoted string without edge-quotes, escape symbol
// but with conversion of escaped characters.
// For example quoted string {"one \"two\"\t three"} transforms to {one "two"		three}
func (t *Token) ValueUnescapedString() string {
	if t.key == TokenString && t.string != nil {
		from := 0
		to := len(t.value)
		if bytesStarts(t.string.StartToken, t.value) {
			from = len(t.string.StartToken)
		}
		if bytesEnds(t.string.EndToken, t.value) {
			to = len(t.value) - len(t.string.EndToken)
		}
		str := t.value[from:to]
		result := make([]byte, 0, len(str))
		escaping := false
		start := 0
		for i := 0; i < len(str); i++ {
			if escaping {
				if v, ok := t.string.SpecSymbols[string(str[i])]; ok {
					result = append(result, t.value[start:i]...)
					result = append(result, v)
				}
				start = i
				escaping = false
			} else if t.string.EscapeSymbol != 0 && str[i] == t.string.EscapeSymbol {
				escaping = true
			}
		}
		if start == 0 { // no one escapes
			return b2s(str)
		} else {
			return b2s(result)
		}
	}
	return ""
}

// Is checks if the token has any of these keys.
func (t *Token) Is(key int, keys ...int) bool {
	if t.key == key {
		return true
	}
	if len(keys) > 0 {
		for _, k := range keys {
			if t.key == k {
				return true
			}
		}
	}
	return false
}
