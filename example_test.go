package tokenizer

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	TokenCurlyOpen TokenKey = iota + 1
	TokenCurlyClose
	TokenSquareOpen
	TokenSquareClose
	TokenColon
	TokenComma
	TokenDoubleQuoted
)

// Example of JSON parser via tokenizer.
// Parse JSON to map.

type jsonParser struct {
	tokenizer *Tokenizer
}

// NewJsonParser create and configure new tokenizer for JSON.
func newJSONParser() *jsonParser {
	parser := &jsonParser{}
	parser.tokenizer = New()
	parser.tokenizer.
		DefineTokens(TokenCurlyOpen, []string{"{"}).
		DefineTokens(TokenCurlyClose, []string{"}"}).
		DefineTokens(TokenSquareOpen, []string{"["}).
		DefineTokens(TokenSquareClose, []string{"]"}).
		DefineTokens(TokenColon, []string{":"}).
		DefineTokens(TokenComma, []string{","}).
		DefineStringToken(TokenDoubleQuoted, `"`, `"`).
		SetEscapeSymbol(BackSlash).AddSpecialStrings(DefaultSpecialString)

	return parser
}

func (parser *jsonParser) Parse(json []byte) (interface{}, error) {
	return parser.analyzer(parser.tokenizer.ParseBytes(json))
}

func (parser *jsonParser) analyzer(stream *Stream) (interface{}, error) {
	if stream.CurrentToken().Is(TokenCurlyOpen) { // analyze objects like {"one": 2, "three": [4, 5]}
		stream.GoNext()
		object := map[string]interface{}{}
		for {
			if stream.CurrentToken().Is(TokenString) { // checks if token is quoted string, then it is object's key
				var key = stream.CurrentToken().ValueUnescapedString()
				var err error
				if stream.GoNext().CurrentToken().Is(TokenColon) { // analyze key's value
					if object[key], err = parser.analyzer(stream.GoNext()); err != nil {
						return nil, err
					}
					if stream.CurrentToken().Is(TokenComma) {
						stream.GoNext()
						if stream.CurrentToken().Is(TokenCurlyClose) {
							return nil, parser.error(stream)
						}
					} else if !stream.CurrentToken().Is(TokenCurlyClose) {
						return nil, parser.error(stream)
					}
				} else {
					return nil, parser.error(stream)
				}
			} else if stream.CurrentToken().Is(TokenCurlyClose) { // checks if token '}', then close the object
				stream.GoNext()
				return object, nil
			} else {
				return nil, parser.error(stream)
			}
		}
	} else if stream.CurrentToken().Is(TokenSquareOpen) { // analyze arrays like [1, "two", {"three": "four"}]
		stream.GoNext()
		array := []interface{}{}
		for {
			if stream.CurrentToken().Is(TokenSquareClose) { // checks if token ']', then close the array
				stream.GoNext()
				return array, nil
			}
			if item, err := parser.analyzer(stream); err == nil { // analyze array's item
				array = append(array, item)
			} else {
				return nil, err
			}
			if stream.CurrentToken().Is(TokenComma) {
				stream.GoNext()
				if stream.CurrentToken().Is(TokenSquareClose) {
					return nil, parser.error(stream)
				}
			} else if !stream.CurrentToken().Is(TokenSquareClose) {
				return nil, parser.error(stream)
			}
		}
	} else if stream.CurrentToken().Is(TokenInteger) { // analyze numbers
		defer stream.GoNext()
		return stream.CurrentToken().ValueInt64(), nil
	} else if stream.CurrentToken().Is(TokenFloat) { // analyze floats
		defer stream.GoNext()
		return stream.CurrentToken().ValueFloat64(), nil
	} else if stream.CurrentToken().Is(TokenString) { // analyze strings
		defer stream.GoNext()
		return stream.CurrentToken().ValueUnescapedString(), nil
	} else {
		return nil, parser.error(stream)
	}
}

func (parser *jsonParser) error(stream *Stream) error {
	if stream.IsValid() {
		return fmt.Errorf("unexpected token %s on line %d near: %s <-- there",
			stream.CurrentToken().value, stream.CurrentToken().line, stream.GetSnippetAsString(5, 0, 0))
	}
	return fmt.Errorf("unexpected end on line %d near: %s <-- there",
		stream.CurrentToken().line, stream.GetSnippetAsString(5, 0, 0))
}

func TestJsonParser(t *testing.T) {
	parser := newJSONParser()

	data, err := parser.Parse([]byte(`{"one": 1, "two": "three", "four": [5, "six", 7.8, -9, -99.9, {}], "five-5": "5-five", "6-six": "six-6"}`))
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{
		"one":  int64(1),
		"two":  "three",
		"four": []interface{}{int64(5), "six", 7.8, int64(-9), -99.9, map[string]interface{}{}},
		"five-5": "5-five",
		"6-six": "six-6",
	}, data)
}
