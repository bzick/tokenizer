package tokenizer

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	TokenCurlyOpen   = 1
	TokenCurlyClose  = 2
	TokenSquareOpen  = 3
	TokenSquareClose = 4
	TokenColon       = 5
	TokenComma       = 6
)

// Example of JSON parser via tokenizer.
// Parse JSON to map.

type JsonParser struct {
	tokenizer *Tokenizer
}

// NewJsonParser create and configure new tokenizer for JSON.
func NewJsonParser() *JsonParser {
	parser := &JsonParser{}
	parser.tokenizer = New()
	parser.tokenizer.
		AddToken(TokenCurlyOpen, []string{"{"}).
		AddToken(TokenCurlyClose, []string{"}"}).
		AddToken(TokenSquareOpen, []string{"["}).
		AddToken(TokenSquareClose, []string{"]"}).
		AddToken(TokenColon, []string{":"}).
		AddToken(TokenComma, []string{","}).
		AddString(`"`, `"`).
		SetEscapeSymbol('\\').
		SetSpecialSymbols(DefaultStringEscapes)

	return parser
}

func (parser *JsonParser) Parse(json []byte) (interface{}, error) {
	return parser.analyzer(parser.tokenizer.ParseBytes(json))
}

func (parser *JsonParser) analyzer(stream *Stream) (interface{}, error) {
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
			} else {
				if item, err := parser.analyzer(stream); err != nil { // analyze array's item
					return nil, err
				} else {
					array = append(array, item)
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
		}
	} else if stream.CurrentToken().Is(TokenInteger) { // analyze numbers
		defer stream.GoNext()
		return stream.CurrentToken().ValueInt(), nil
	} else if stream.CurrentToken().Is(TokenFloat) { // analyze floats
		defer stream.GoNext()
		return stream.CurrentToken().ValueFloat(), nil
	} else if stream.CurrentToken().Is(TokenString) { // analyze strings
		defer stream.GoNext()
		return stream.CurrentToken().ValueUnescapedString(), nil
	} else {
		return nil, parser.error(stream)
	}
}

func (parser *JsonParser) error(stream *Stream) error {
	if stream.IsValid() {
		return fmt.Errorf("unexpected token %s on line %d near: %s <-- there",
			stream.CurrentToken().value, stream.CurrentToken().line, stream.GetSegmentAsString(5, 0, 0))
	} else {
		return fmt.Errorf("unexpected end on line %d near: %s <-- there",
			stream.CurrentToken().line, stream.GetSegmentAsString(5, 0, 0))
	}
}

func TestJsonParser(t *testing.T) {
	parser := NewJsonParser()

	data, err := parser.Parse([]byte(`{"one": 1, "two": "three", "four": [5, "six", 7.8, {}]}`))
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{
		"one":  int64(1),
		"two":  "three",
		"four": []interface{}{int64(5), "six", 7.8, map[string]interface{}{}},
	}, data)
}

func BenchmarkSandbox(b *testing.B) {
	a := make([]int, 1000)
	max := 0
	for i := 0; i < b.N; i++ {
		t := make([]int, 1000)
		for j := 0; j < 1000; j++ {
			t[j] = j
		}
		a = append(a, t...)
		a = a[1000:]
		if max < cap(a) {
			max = cap(a)
		}
	}
	b.Logf("%d) length %d, capacity %d (max %d)", b.N, len(a), cap(a), max)
}
