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
	condTokenKey := TokenKey(10)
	wordTokenKey := TokenKey(11)
	openKey := TokenKey(12)
	closeKey := TokenKey(13)
	dquoteKey := TokenKey(14)
	// tokenizer.AllowKeywordUnderscore()
	// tokenizer.AllowNumbersInKeyword()
	tokenizer.AllowKeywordSymbols(Underscore, Numbers)
	tokenizer.DefineTokens(condTokenKey, []string{">=", "<=", "==", ">", "<"})
	tokenizer.DefineTokens(wordTokenKey, []string{"or", "или"})
	tokenizer.DefineTokens(openKey, []string{"{{"})
	tokenizer.DefineTokens(closeKey, []string{"}}"})
	tokenizer.DefineStringToken(dquoteKey, `"`, `"`).SetEscapeSymbol('\\').AddInjection(openKey, closeKey)

	str := `field_a > 10 "value1" 12.3 "value2 {{ value3 }} value4"`
	stream := tokenizer.ParseString(str)

	require.Equal(t, len(str), stream.GetParsedLength())

	require.True(t, stream.IsValid())
	require.True(t, stream.NextToken().IsValid())
	require.Equal(t, TokenKeyword, stream.CurrentToken().Key())
	require.True(t, stream.CurrentToken().IsKeyword())
	require.False(t, stream.CurrentToken().IsFloat())
	require.False(t, stream.CurrentToken().IsInteger())
	require.False(t, stream.CurrentToken().IsNumber())
	require.False(t, stream.CurrentToken().IsString())
	require.Equal(t, []byte("field_a"), stream.CurrentToken().Value())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt64())
	require.Equal(t, "field_a", stream.CurrentToken().ValueUnescapedString())
	require.Equal(t, []byte(nil), stream.CurrentToken().Indent())
	require.True(t, stream.IsNextSequence(condTokenKey, TokenInteger, TokenString, TokenFloat))
	require.False(t, stream.IsNextSequence(condTokenKey, TokenInteger, TokenString, TokenInteger))
	require.False(t, stream.IsNextSequence(condTokenKey, TokenFloat, TokenString, TokenFloat))
	require.False(t, stream.IsNextSequence(condTokenKey, TokenInteger, TokenFloat, TokenFloat))
	require.True(t, stream.IsAnyNextSequence(
		[]TokenKey{condTokenKey},
		[]TokenKey{TokenInteger},
		[]TokenKey{TokenString},
		[]TokenKey{TokenFloat, TokenInteger}))
	require.False(t, stream.IsAnyNextSequence(
		[]TokenKey{condTokenKey},
		[]TokenKey{TokenInteger},
		[]TokenKey{TokenString},
		[]TokenKey{TokenString, TokenInteger}))

	require.Equal(t, condTokenKey, stream.NextToken().Key())
	require.Equal(t, []byte(">"), stream.NextToken().Value())

	stream.GoNext()

	require.True(t, stream.IsValid())
	require.True(t, stream.CurrentToken().IsValid())
	require.Equal(t, condTokenKey, stream.CurrentToken().Key())
	require.Equal(t, []byte(">"), stream.CurrentToken().Value())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt64())
	require.Equal(t, float64(0.0), stream.CurrentToken().ValueFloat64())
	require.Equal(t, ">", stream.CurrentToken().ValueUnescapedString())
	require.Equal(t, []byte(" "), stream.CurrentToken().Indent())

	require.False(t, stream.GoNextIfNextIs(TokenKeyword))
	require.True(t, stream.GoNextIfNextIs(TokenInteger))

	require.Equal(t, TokenInteger, stream.CurrentToken().Key())
	require.False(t, stream.CurrentToken().IsKeyword())
	require.False(t, stream.CurrentToken().IsFloat())
	require.True(t, stream.CurrentToken().IsInteger())
	require.True(t, stream.CurrentToken().IsNumber())
	require.False(t, stream.CurrentToken().IsString())
	require.Equal(t, int64(10), stream.CurrentToken().ValueInt64())
	require.Equal(t, float64(10.0), stream.CurrentToken().ValueFloat64())
	require.Equal(t, "10", stream.CurrentToken().ValueUnescapedString())

	stream.GoNext()

	require.Equal(t, TokenString, stream.CurrentToken().Key())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt64())
	require.Equal(t, float64(0), stream.CurrentToken().ValueFloat64())
	require.Equal(t, "value1", stream.CurrentToken().ValueUnescapedString())

	stream.GoTo(7)

	require.Equal(t, "value3", stream.CurrentToken().ValueUnescapedString())
	require.Equal(t, TokenKeyword, stream.CurrentToken().Key())
	require.Equal(t, int64(0), stream.CurrentToken().ValueInt64())
	require.Equal(t, float64(0), stream.CurrentToken().ValueFloat64())

	stream.Close()
}

func TestHistory(t *testing.T) {
	tokenizer := New()
	tokens := tokenizer.ParseString("0 1 2 3 4 5 6 7 8 9")
	tokens.SetHistorySize(3)

	require.Equal(t, 0, tokens.CurrentToken().ID())
	require.Equal(t, int64(0), tokens.CurrentToken().ValueInt64())
	require.Equal(t, 0, tokens.HeadToken().ID())
	require.Equal(t, int64(0), tokens.HeadToken().ValueInt64())
	require.Equal(t, 10, tokens.len)

	tokens.GoNext()
	tokens.GoNext()

	require.Equal(t, 2, tokens.CurrentToken().ID())
	require.Equal(t, int64(2), tokens.CurrentToken().ValueInt64())
	require.Equal(t, 0, tokens.HeadToken().ID())
	require.Equal(t, int64(0), tokens.HeadToken().ValueInt64())
	require.Equal(t, 10, tokens.len)

	tokens.GoNext()
	tokens.GoNext()

	require.Equal(t, 4, tokens.CurrentToken().ID())
	require.Equal(t, int64(4), tokens.CurrentToken().ValueInt64())
	require.Equal(t, 1, tokens.HeadToken().ID())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt64())
	require.Equal(t, 9, tokens.len)

	tokens.GoPrev()
	tokens.GoPrev()
	tokens.GoPrev()

	require.Equal(t, 1, tokens.CurrentToken().ID())
	require.Equal(t, int64(1), tokens.CurrentToken().ValueInt64())
	require.Equal(t, 1, tokens.HeadToken().ID())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt64())
	require.Equal(t, 9, tokens.len)

	tokens.GoPrev()

	require.Equal(t, -1, tokens.CurrentToken().ID())
	require.Equal(t, int64(0), tokens.CurrentToken().ValueInt64())
	require.Equal(t, 1, tokens.HeadToken().ID())
	require.Equal(t, int64(1), tokens.HeadToken().ValueInt64())
	require.Equal(t, 9, tokens.len)
}

func TestSequenceWithHistory(t *testing.T) {
	tokenizer := New()
	tokens := tokenizer.ParseString("0 1.1 2 3.3 4 5.5 6")
	tokens.SetHistorySize(3)

	require.True(t, tokens.IsNextSequence(TokenFloat, TokenInteger, TokenFloat, TokenInteger, TokenFloat, TokenInteger))
	require.Equal(t, 0, tokens.CurrentToken().ID())
	require.Equal(t, 3, tokens.historySize)

	require.True(t, tokens.IsAnyNextSequence(
		[]TokenKey{TokenFloat},
		[]TokenKey{TokenInteger, TokenFloat},
		[]TokenKey{TokenFloat},
		[]TokenKey{TokenInteger, TokenFloat},
		[]TokenKey{TokenFloat},
		[]TokenKey{TokenInteger, TokenFloat}))
	require.Equal(t, 0, tokens.CurrentToken().ID())
	require.Equal(t, 3, tokens.historySize)

}

func TestInfStream(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	for i := 0; i < 100; i++ {
		buffer.Write([]byte(fmt.Sprintf(`{id: %d, key: "object number %d"}`, i, i)))
	}

	tokenizer := New()
	commaKey := TokenKey(10)
	colonKey := TokenKey(11)
	openKey := TokenKey(12)
	closeKey := TokenKey(13)
	dquoteKey := TokenKey(14)
	tokenizer.DefineTokens(commaKey, []string{","})
	tokenizer.DefineTokens(colonKey, []string{":"})
	tokenizer.DefineTokens(openKey, []string{"{"})
	tokenizer.DefineTokens(closeKey, []string{"}"})
	tokenizer.DefineStringToken(dquoteKey, `"`, `"`).SetEscapeSymbol('\\')

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
		id := stream.CurrentToken().ValueInt64()
		stream.GoNext()

		require.True(t, stream.CurrentToken().Is(commaKey))
		stream.GoNext()

		require.Truef(t, stream.CurrentToken().Is(TokenKeyword), "iteration %d: %s", n, stream.GetSnippetAsString(10, 10, 10))
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

func TestIssues13_SequenceLongerThenStream(t *testing.T) {
	const TOK_CMD = 100
	var parser = New()
	parser.DefineTokens(TOK_CMD, []string{"CMD"})

	stream := parser.ParseString("CMD CMD")

	ok := stream.IsNextSequence(TOK_CMD, TOK_CMD, TOK_CMD, TOK_CMD)
	require.False(t, ok)
}

func TestIssue11(t *testing.T) {
	parser := New()
	parser.AllowKeywordSymbols(nil, Numbers)
	parser.DefineTokens(1, []string{".."})

	stream := parser.ParseString("1..2")
	require.Equal(t, "1", string(stream.CurrentToken().Value()))
	stream.GoNext()
	require.Equal(t, "..", string(stream.CurrentToken().Value()))
	stream.GoNext()
	require.Equal(t, "2", string(stream.CurrentToken().Value()))
}

func TestIssue9(t *testing.T) {
	parser := New()
	buf := bytes.NewBuffer([]byte("a]"))

	stream := parser.ParseStream(buf, 4096)
	defer stream.Close()
	stream.CurrentToken()
	for stream.IsValid() {
		switch stream.CurrentToken().Key() {
		default:
			// println(stream.CurrentToken().ValueString())
			stream.GoNext()
		}
	}
}

var pattern = []byte(`<item count=10 valid id="n9762"> Носки <![CDATA[ socks ]]></item>`)

type dataGenerator struct {
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
	tagOpen := TokenKey(1)
	tagClose := TokenKey(2)
	equal := TokenKey(3)
	slash := TokenKey(4)
	dquote := TokenKey(5)
	cdata := TokenKey(6)
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
		stream.GoNext()
	}

	dif := time.Since(t)
	b.Logf("Speed: %d bytes at %s: %d byte/sec", len(reader.data), dif, int(float64(len(reader.data))/dif.Seconds()))
}

func BenchmarkParseBytes(b *testing.B) {
	reader := newDataGenerator(b.N)
	tokenizer := New()
	tagOpen := TokenKey(1)
	tagClose := TokenKey(2)
	equal := TokenKey(3)
	slash := TokenKey(4)
	dquote := TokenKey(5)
	cdata := TokenKey(6)
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

	dif := time.Since(t)
	size := len(reader.data)
	b.Logf("Speed: %d bytes string with %s: %d byte/sec", size, dif, int(float64(size)/dif.Seconds()))
}

func BenchmarkMap(b *testing.B) {
	mp := map[byte]bool{
		'0': true,
		'1': true,
		'2': true,
		'3': true,
		'4': true,
		'5': true,
		'6': true,
		'7': true,
		'8': true,
		'9': true,
	}

	var x bool

	for i := 0; i < b.N; i++ {
		x = mp['9']
	}

	_ = x
}

func BenchmarkSlice(b *testing.B) {
	s := []byte{
		'0',
		'1',
		'2',
		'3',
		'4',
		'5',
		'6',
		'7',
		'8',
		'9',
		'A',
		'B',
		'C',
		'D',
		'E',
		'F',
	}

	for i := 0; i < b.N; i++ {
		for _, v := range s {
			if v == 'F' {
				break
			}
		}
	}

}
