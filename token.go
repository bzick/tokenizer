package tokenizer

import (
	"bytes"
	"fmt"
	"strconv"
)

var undefToken = &Token{
	id: -1,
}

// Token struct describe one token.
type Token struct {
	id     int
	key    TokenKey
	value  []byte
	line   int
	offset int
	indent []byte
	string *StringSettings

	prev *Token
	next *Token
}

// addNext add new token as next node of dl-list.
func (t *Token) addNext(next *Token) {
	next.prev = t
	t.next = next
}

// unlink remove token from dl-list and fix links of prev and next nodes.
// Method returns next token or nil if no next token found.
func (t *Token) unlink() *Token {
	next := t.next
	t.next.prev = nil
	t.next = nil
	t.prev = nil

	return next
}

// ID returns id of token. Id is the sequence number of tokens in the stream.
func (t *Token) ID() int {
	return t.id
}

// String returns a multiline string with the token's information.
func (t *Token) String() string {
	return fmt.Sprintf("{\n\tId: %d\n\tKey: %d\n\tValue: %s\n\tPosition: %d\n\tIndent: %d bytes\n\tLine: %d\n}",
		t.id, t.key, t.value, t.offset, len(t.indent), t.line)
}

// IsValid checks if this token is valid — the key is not TokenUndef.
func (t *Token) IsValid() bool {
	return t.key != TokenUndef
}

// IsKeyword checks if this is keyword — the key is TokenKeyword.
func (t *Token) IsKeyword() bool {
	return t.key == TokenKeyword
}

// IsNumber checks if this token is integer or float — the key is TokenInteger or TokenFloat.
func (t *Token) IsNumber() bool {
	return t.key == TokenInteger || t.key == TokenFloat
}

// IsFloat checks if this token is float — the key is TokenFloat.
func (t *Token) IsFloat() bool {
	return t.key == TokenFloat
}

// IsInteger checks if this token is integer — the key is TokenInteger.
func (t *Token) IsInteger() bool {
	return t.key == TokenInteger
}

// ValueInt64 returns value as int64.
// If the token is float the result wild be round by math's rules.
// If the token is not TokenInteger or TokenFloat then method returns zero
// Method doesn't use cache — each call starts a number parser.
func (t *Token) ValueInt64() int64 {
	if t.key == TokenInteger {
		num, _ := strconv.ParseInt(b2s(t.value), 0, 64)
		return num
	} else if t.key == TokenFloat {
		num, _ := strconv.ParseFloat(b2s(t.value), 64)
		return int64(num)
	}
	return 0
}

// Deprecated: use ValueInt64
func (t *Token) ValueInt() int64 {
	return t.ValueInt64()
}

// ValueFloat64 returns value as float64.
// If the token is not TokenInteger or TokenFloat then method returns zero.
// Method doesn't use cache — each call starts a number parser.
func (t *Token) ValueFloat64() float64 {
	if t.key == TokenFloat {
		num, _ := strconv.ParseFloat(b2s(t.value), 64)
		return num
	} else if t.key == TokenInteger {
		num, _ := strconv.ParseInt(b2s(t.value), 0, 64)
		return float64(num)
	}
	return 0.0
}

// Deprecated: use ValueFloat64
func (t *Token) ValueFloat() float64 {
	return t.ValueFloat64()
}

// Indent returns spaces before the token.
func (t *Token) Indent() []byte {
	return t.indent
}

// Key returns the key of the token pointed to by the pointer.
// If pointer is not valid (see IsValid) TokenUndef will be returned.
func (t *Token) Key() TokenKey {
	return t.key
}

// Value returns value of current token as slice of bytes from source.
// If current token is invalid value returns nil.
//
// Do not change bytes in the slice. Copy slice before change.
func (t *Token) Value() []byte {
	return t.value
}

// ValueString returns value of the token as string.
// If the token is TokenUndef method returns empty string.
func (t *Token) ValueString() string {
	if t.value == nil {
		return ""
	}
	return b2s(t.value)
}

// Line returns line number in input string.
// Line numbers starts from 1.
func (t *Token) Line() int {
	return t.line
}

// Offset returns the byte position in input string (from start).
func (t *Token) Offset() int {
	return t.offset
}

// StringSettings returns StringSettings structure if token is framed string.
func (t *Token) StringSettings() *StringSettings {
	return t.string
}

// StringKey returns key of string.
// If key not defined for string TokenString will be returned.
func (t *Token) StringKey() TokenKey {
	if t.string != nil {
		return t.string.Key
	}
	return TokenString
}

// IsString checks if current token is a quoted string.
// Token key may be TokenString or TokenStringFragment.
func (t *Token) IsString() bool {
	return t.key == TokenString || t.key == TokenStringFragment
}

// ValueUnescaped returns clear (unquoted) string
//   - without edge-tokens (quotes)
//   - with character escaping handling
//
// For example quoted string
//
//	"one \"two\"\t three"
//
// transforms to
//
//	one "two"		three
//
// Method doesn't use cache. Each call starts a string parser.
func (t *Token) ValueUnescaped() []byte {
	if t.string != nil {
		from := 0
		to := len(t.value)
		if bytesStarts(t.string.StartToken, t.value) {
			from = len(t.string.StartToken)
		}
		if bytesEnds(t.string.EndToken, t.value) {
			to = len(t.value) - len(t.string.EndToken)
		}
		str := t.value[from:to]
		var result []byte
		for len(str) > 0 {
			if idx := bytes.IndexByte(str, t.string.EscapeSymbol); idx != -1 {
				if p := hasAnyPrefix(t.string.SpecSymbols, str[idx+1:]); p != nil {
					result = append(result, str[:idx]...)
					result = append(result, p...)
					str = str[idx+len(p)+1:]
				} else {
					break
				}
			} else {
				break
			}
		}
		if result == nil {
			return str
		}

		if len(str) > 0 {
			result = append(result, str...)
		}
		return result
	}
	return t.value
}

// ValueUnescapedString like as ValueUnescaped but returns string.
func (t *Token) ValueUnescapedString() string {
	if s := t.ValueUnescaped(); s != nil {
		return b2s(s)
	}
	return ""
}

// Is checks if the token has any of these keys.
func (t *Token) Is(key TokenKey, keys ...TokenKey) bool {
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

func hasAnyPrefix(prefixes [][]byte, where []byte) []byte {
	for _, w := range prefixes {
		if bytes.HasPrefix(where, w) {
			return w
		}
	}
	return nil
}

func runeExists(s []rune, v rune) bool {
	for _, val := range s {
		if val == v {
			return true
		}
	}
	return false
}
