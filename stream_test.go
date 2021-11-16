package tokenizer

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestStream(t *testing.T) {
	tokenizer := New()
	condTokenKey := 10
	wordTokenKey := 11
	openKey := 12
	closeKey := 13
	dquoteKey := 14
	tokenizer.AllowKeywordUnderscore()
	tokenizer.DefineTokens(condTokenKey, []string{">=", "<=", "==", ">", "<"})
	tokenizer.DefineTokens(wordTokenKey, []string{"or", "или"})
	tokenizer.DefineTokens(openKey, []string{"{{"})
	tokenizer.DefineTokens(closeKey, []string{"}}"})
	tokenizer.DefineStringToken(dquoteKey, `"`, `"`).SetEscapeSymbol('\\').AddInjection(openKey, closeKey)

	str := `field_a > 10 "value1" 12.3 "value2 {{ value3 }} value4"`
	stream := tokenizer.ParseString(str)
	require.True(t, stream.IsValid())
	require.True(t, stream.NextToken().IsValid())
	require.Equal(t, TokenKeyword, stream.CurrentToken().Key())
	require.Equal(t, []byte("field_a"), stream.CurrentToken().Value())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt())
	require.Equal(t, "field_a", stream.CurrentToken().ValueUnescapedString())
	require.Equal(t, []byte(nil), stream.CurrentToken().Indent())

	require.Equal(t, condTokenKey, stream.NextToken().Key())
	require.Equal(t, []byte(">"), stream.NextToken().Value())

	stream.Next()

	require.True(t, stream.IsValid())
	require.True(t, stream.CurrentToken().IsValid())
	require.Equal(t, condTokenKey, stream.CurrentToken().Key())
	require.Equal(t, []byte(">"), stream.CurrentToken().Value())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt())
	require.Equal(t, float64(0.0), stream.CurrentToken().ValueFloat())
	require.Equal(t, ">", stream.CurrentToken().ValueUnescapedString())
	require.Equal(t, []byte(" "), stream.CurrentToken().Indent())

	require.False(t, stream.GoNextIfNextIs(TokenKeyword))
	require.True(t, stream.GoNextIfNextIs(TokenInteger))

	require.Equal(t, TokenInteger, stream.CurrentToken().Key())
	require.Equal(t, int64(10), stream.CurrentToken().ValueInt())
	require.Equal(t, float64(10.0), stream.CurrentToken().ValueFloat())
	require.Equal(t, "10", stream.CurrentToken().ValueUnescapedString())

	stream.Next()

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

	tokens.Next()
	tokens.Next()

	require.Equal(t, 2, tokens.CurrentToken().Id())
	require.Equal(t, int64(2), tokens.CurrentToken().ValueInt())
	require.Equal(t, 0, tokens.HeadToken().Id())
	require.Equal(t, int64(0), tokens.HeadToken().ValueInt())
	require.Equal(t, 10, tokens.len)

	tokens.Next()
	tokens.Next()

	require.Equal(t, 4, tokens.CurrentToken().Id())
	require.Equal(t, int64(4), tokens.CurrentToken().ValueInt())
	require.Equal(t, 1, tokens.HeadToken().Id())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt())
	require.Equal(t, 9, tokens.len)

	tokens.Prev()
	tokens.Prev()
	tokens.Prev()

	require.Equal(t, 1, tokens.CurrentToken().Id())
	require.Equal(t, int64(1), tokens.CurrentToken().ValueInt())
	require.Equal(t, 1, tokens.HeadToken().Id())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt())
	require.Equal(t, 9, tokens.len)

	tokens.Prev()

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
	dquoteKey := 14
	tokenizer.DefineTokens(commaKey, []string{","})
	tokenizer.DefineTokens(colonKey, []string{":"})
	tokenizer.DefineTokens(openKey, []string{"{"})
	tokenizer.DefineTokens(closeKey, []string{"}"})
	tokenizer.DefineStringToken(dquoteKey, `"`, `"`).SetEscapeSymbol('\\')

	stream := tokenizer.ParseStream(buffer, 100).SetHistorySize(100)

	n := 0
	for stream.IsValid() {
		require.True(t, stream.CurrentToken().Is(openKey))
		stream.Next()

		require.True(t, stream.CurrentToken().Is(TokenKeyword))
		require.Equal(t, []byte("id"), stream.CurrentToken().Value())
		stream.Next()

		require.True(t, stream.CurrentToken().Is(colonKey))
		stream.Next()

		require.True(t, stream.CurrentToken().Is(TokenInteger))
		id := stream.CurrentToken().ValueInt()
		stream.Next()

		require.True(t, stream.CurrentToken().Is(commaKey))
		stream.Next()

		require.Truef(t, stream.CurrentToken().Is(TokenKeyword), "iteration %d: %s", n, stream.GetSnippetAsString(10, 10, 10))
		require.Equal(t, []byte("key"), stream.CurrentToken().Value())
		stream.Next()

		require.True(t, stream.CurrentToken().Is(colonKey))
		stream.Next()

		require.True(t, stream.CurrentToken().Is(TokenString))
		require.Equal(t, fmt.Sprintf("object number %d", id), stream.CurrentToken().ValueUnescapedString())
		stream.Next()

		require.True(t, stream.CurrentToken().Is(closeKey))

		stream.Next()
		n++
		if n >= 100 {
			break
		}

	}
	require.Equal(t, 100, n)

}

var pattern = []byte(`<item count=10 valid id="n9762"> Носки <![CDATA[ socks ]]></item>`)

type dataGenerator struct {
	size int
	i    int
	data []byte
}

func newDataGenerator(size int) *dataGenerator {
	var data = make([]byte, len(pattern)*size)
	for j := 0; j < size; j++ {
		copy(data[len(pattern)*j:], pattern)
	}
	return &dataGenerator{
		data: data,
	}
}

func (d *dataGenerator) Read(p []byte) (n int, err error) {
	from := d.i
	to := d.i + len(p)
	if to > len(d.data) {
		to = len(d.data)
	}
	copy(p, d.data[from:to])
	d.i = to
	return to - from, nil
}

func BenchmarkParseInfStream(b *testing.B) {
	reader := newDataGenerator(b.N)
	tokenizer := New()
	tagOpen := 1
	tagClose := 2
	equal := 3
	slash := 4
	dquote := 5
	cdata := 6
	tokenizer.DefineTokens(tagOpen, []string{"<"})
	tokenizer.DefineTokens(tagClose, []string{">"})
	tokenizer.DefineTokens(equal, []string{"="})
	tokenizer.DefineTokens(slash, []string{"/"})
	tokenizer.DefineStringToken(dquote, `"`, `"`).SetEscapeSymbol('\\')
	tokenizer.DefineStringToken(cdata, `<![CDATA[`, `]]>`)

	b.ResetTimer()
	t := time.Now()
	stream := tokenizer.ParseStream(reader, 4096).SetHistorySize(10)

	for stream.IsValid() {
		stream.Next()
	}

	dif := time.Now().Sub(t)
	b.Logf("Speed: %d bytes at %s: %d byte/sec", len(reader.data), dif, int(float64(len(reader.data))/dif.Seconds()))
}

func BenchmarkParseBytes(b *testing.B) {
	reader := newDataGenerator(b.N)
	tokenizer := New()
	tagOpen := 1
	tagClose := 2
	equal := 3
	slash := 4
	dquote := 5
	cdata := 6
	tokenizer.DefineTokens(tagOpen, []string{"<"})
	tokenizer.DefineTokens(tagClose, []string{">"})
	tokenizer.DefineTokens(equal, []string{"="})
	tokenizer.DefineTokens(slash, []string{"/"})
	tokenizer.DefineStringToken(dquote, `"`, `"`).SetEscapeSymbol('\\')
	tokenizer.DefineStringToken(cdata, `<![CDATA[`, `]]>`)

	b.ResetTimer()

	t := time.Now()
	stream := tokenizer.ParseBytes(reader.data)
	stream.IsValid()

	dif := time.Now().Sub(t)
	size := len(reader.data)
	b.Logf("Speed: %d bytes string with %s: %d byte/sec", size, dif, int(float64(size)/dif.Seconds()))
}
