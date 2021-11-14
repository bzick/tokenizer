package tokenizer

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStream(t *testing.T) {
	tokenizer := New()
	condTokenKey := 10
	wordTokenKey := 11
	openKey := 12
	closeKey := 13
	tokenizer.AllowKeywordUnderscore()
	tokenizer.AddToken(condTokenKey, []string{">=", "<=", "==", ">", "<"})
	tokenizer.AddToken(wordTokenKey, []string{"or", "или"})
	tokenizer.AddToken(openKey, []string{"{{"})
	tokenizer.AddToken(closeKey, []string{"}}"})
	tokenizer.AddString(`"`, `"`).SetEscapeSymbol('\\').AddInjection(openKey, closeKey)

	str := `field_a > 10 "value1" 12.3 "value2 {{ value3 }} value4"`
	stream := tokenizer.ParseString(str)
	require.True(t, stream.IsValid())
	require.True(t, stream.NextToken().IsValid())
	require.Equal(t, TokenKeyword, stream.CurrentToken().Key())
	require.Equal(t, []byte("field_a"), stream.CurrentToken().Value())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt())
	require.Equal(t, "", stream.CurrentToken().ValueUnescapedString())
	require.Equal(t, []byte(nil), stream.CurrentToken().Indent())

	require.Equal(t, condTokenKey, stream.NextToken().Key())
	require.Equal(t, []byte(">"), stream.NextToken().Value())

	stream.GoNext()

	require.True(t, stream.IsValid())
	require.True(t, stream.CurrentToken().IsValid())
	require.Equal(t, condTokenKey, stream.CurrentToken().Key())
	require.Equal(t, []byte(">"), stream.CurrentToken().Value())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt())
	require.Equal(t, float64(0.0), stream.CurrentToken().ValueFloat())
	require.Equal(t, "", stream.CurrentToken().ValueUnescapedString())
	require.Equal(t, []byte(" "), stream.CurrentToken().Indent())

	require.False(t, stream.GoNextIfNextIs(TokenKeyword))
	require.True(t, stream.GoNextIfNextIs(TokenInteger))

	require.Equal(t, TokenInteger, stream.CurrentToken().Key())
	require.Equal(t, int64(10), stream.CurrentToken().ValueInt())
	require.Equal(t, float64(10.0), stream.CurrentToken().ValueFloat())
	require.Equal(t, "", stream.CurrentToken().ValueUnescapedString())

	stream.GoNext()

	require.Equal(t, TokenString, stream.CurrentToken().Key())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt())
	require.Equal(t, float64(0), stream.CurrentToken().ValueFloat())
	require.Equal(t, "value1", stream.CurrentToken().ValueUnescapedString())
}

func TestHistory(t *testing.T) {
	tokenizer := New()
	tokens := tokenizer.ParseString("0 1 2 3 4 5 6 7 8 9")
	tokens.SetHistorySize(3)

	require.Equal(t, 0, tokens.CurrentToken().Id())
	require.Equal(t, int64(0), tokens.CurrentToken().ValueInt())
	require.Equal(t, 0, tokens.HeadToken().Id())
	require.Equal(t, int64(0), tokens.HeadToken().ValueInt())
	require.Equal(t, 10, tokens.len)

	tokens.GoNext()
	tokens.GoNext()

	require.Equal(t, 2, tokens.CurrentToken().Id())
	require.Equal(t, int64(2), tokens.CurrentToken().ValueInt())
	require.Equal(t, 0, tokens.HeadToken().Id())
	require.Equal(t, int64(0), tokens.HeadToken().ValueInt())
	require.Equal(t, 10, tokens.len)

	tokens.GoNext()
	tokens.GoNext()

	require.Equal(t, 4, tokens.CurrentToken().Id())
	require.Equal(t, int64(4), tokens.CurrentToken().ValueInt())
	require.Equal(t, 1, tokens.HeadToken().Id())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt())
	require.Equal(t, 9, tokens.len)

	tokens.GoPrev()
	tokens.GoPrev()
	tokens.GoPrev()

	require.Equal(t, 1, tokens.CurrentToken().Id())
	require.Equal(t, int64(1), tokens.CurrentToken().ValueInt())
	require.Equal(t, 1, tokens.HeadToken().Id())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt())
	require.Equal(t, 9, tokens.len)

	tokens.GoPrev()

	require.Equal(t, -1, tokens.CurrentToken().Id())
	require.Equal(t, int64(0), tokens.CurrentToken().ValueInt())
	require.Equal(t, 1, tokens.HeadToken().Id())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt())
	require.Equal(t, 9, tokens.len)
}

func TestInfStream(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	for i := 0; i < 100; i++ {
		buffer.Write([]byte(fmt.Sprintf(`{id: %d, key: "object number %d"}`, i, i)))
	}

	tokenizer := New()
	commaKey := 10
	colonKey := 11
	openKey := 12
	closeKey := 13
	tokenizer.AddToken(commaKey, []string{","})
	tokenizer.AddToken(colonKey, []string{":"})
	tokenizer.AddToken(openKey, []string{"{"})
	tokenizer.AddToken(closeKey, []string{"}"})
	tokenizer.AddString(`"`, `"`).SetEscapeSymbol('\\')

	stream := tokenizer.ParseStream(buffer, 100).SetHistorySize(100)

	n := 0
	for stream.IsValid() {
		require.True(t, stream.CurrentToken().Is(openKey))
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(TokenKeyword))
		require.Equal(t, []byte("id"), stream.CurrentToken().Value())
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(colonKey))
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(TokenInteger))
		id := stream.CurrentToken().ValueInt()
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(commaKey))
		stream.GoNext()

		require.Truef(t, stream.CurrentToken().Is(TokenKeyword), "iteration %d: %s", n, stream.GetSegmentAsString(10, 10, 10))
		require.Equal(t, []byte("key"), stream.CurrentToken().Value())
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(colonKey))
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(TokenString))
		require.Equal(t, fmt.Sprintf("object number %d", id), stream.CurrentToken().ValueUnescapedString())
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(closeKey))

		stream.GoNext()
		n++
		if n >= 100 {
			break
		}

	}
	require.Equal(t, 100, n)

}
