package tokenizer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/require"
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

// TestIssue26 panic when stream with more than one token ends with any white space
func TestIssue26(t *testing.T) {
	parser := New()
	buf := bytes.NewBufferString("test coucou\n")

	stream := parser.ParseStream(buf, 4096)
	defer stream.Close()
	for stream.IsValid() {
		// do something...
		stream.GoNext() // Should not panic
	}
}

// TestIssue30 second use of ParseStream() after a first closed empty stream,
// produces an incorrectly invalid stream.
func TestIssue30(t *testing.T) {
	parser := New()

	// first stream is empty
	buf1 := bytes.NewBufferString("")
	stream1 := parser.ParseStream(buf1, 4096)
	for stream1.IsValid() {
		t.Error("stream1 should have been invalid from the beginning")
	}
	// closing the first stream before opening the new one
	stream1.Close()

	// second stream created from the same parser
	buf2 := bytes.NewBufferString("hello world")
	stream2 := parser.ParseStream(buf2, 4096)
	defer stream2.Close()
	if !stream2.IsValid() {
		t.Fatal("stream2 should be valid")
	}
}

// TestIssue44 The parser stops parsing the stream after the first token if
// its size greater or equal to the size of the parser's buffer.
func TestIssue44(t *testing.T) {
	parser := New()
	parser.AllowKeywordSymbols(Underscore, Numbers)

	// stream with first token close to the size of the parser's internal buffer
	buf := bytes.NewBufferString("this_token_is_exactly_40_characters_long 67")
	stream := parser.ParseStream(buf, 40)
	defer stream.Close()

	require.True(t, stream.IsValid())
	require.True(t, stream.CurrentToken().IsKeyword())
	require.Equal(t, "this_token_is_exactly_40_characters_long", stream.CurrentToken().ValueString())

	stream.GoNext()
	require.True(t, stream.IsValid(), "stream should still be valid after first token")
	require.True(t, stream.CurrentToken().IsInteger())
	require.Equal(t, int64(67), stream.CurrentToken().ValueInt64())
}

// TestIssue44NextToken NextToken should be able to peek the next token even
// when the first token's size is grreater or equal to the size of the parser's
// buffer.
func TestIssue44NextToken(t *testing.T) {
	parser := New()
	parser.AllowKeywordSymbols(Underscore, Numbers)

	// stream with first token close to the size of the parser's internal buffer
	buf := bytes.NewBufferString("this_token_is_exactly_40_characters_long 67")
	stream := parser.ParseStream(buf, 40)
	defer stream.Close()

	require.True(t, stream.IsValid())
	next := stream.NextToken()
	require.NotEqual(t, undefToken, next, "NextToken should not return undefToken")
	require.True(t, next.IsInteger())
	require.Equal(t, int64(67), next.ValueInt64())
}

func TestStreamOverflow(t *testing.T) {
	parser := New()
	buf := bytes.NewBuffer([]byte("a b c"))

	stream := parser.ParseStream(buf, 4096)
	defer stream.Close()

	require.False(t, stream.IsNextSequence(TokenKeyword, TokenKeyword, TokenKeyword, TokenKeyword))
	require.False(t, stream.IsNextSequence(TokenKeyword, TokenKeyword, TokenKeyword, TokenKeyword))
}

// streamToken is a comparable copy of a token without the links of the token list.
type streamToken struct {
	id     int
	key    TokenKey
	value  string
	line   int
	offset int
	indent string
	string *StringSettings
}

func collectTokens(stream *Stream) []streamToken {
	var tokens []streamToken
	for stream.IsValid() {
		c := stream.CurrentToken()
		tokens = append(tokens, streamToken{c.id, c.key, string(c.value), c.line, c.offset, string(c.indent), c.string})
		stream.GoNext()
	}
	return tokens
}

// TestStreamLargerThanChunk parses a stream that is many times larger than the parser's buffer.
func TestStreamLargerThanChunk(t *testing.T) {
	const commaKey = TokenKey(1)
	parser := New()
	parser.DefineTokens(commaKey, []string{","})

	for _, repeat := range []int{5, 5000} {
		for _, size := range []uint{4, 16, 64, 4096} {
			stream := parser.ParseStream(strings.NewReader(strings.Repeat("a, ", repeat)), size)
			n := 0
			for ; stream.IsValid(); n++ {
				if n%2 == 0 {
					require.Truef(t, stream.CurrentToken().IsKeyword(), "repeat %d, buffer size %d, token %d", repeat, size, n)
					require.Equal(t, "a", stream.CurrentToken().ValueString())
				} else {
					require.Truef(t, stream.CurrentToken().Is(commaKey), "repeat %d, buffer size %d, token %d", repeat, size, n)
				}
				stream.GoNext()
			}
			require.Equalf(t, 2*repeat, n, "repeat %d, buffer size %d", repeat, size)
			stream.Close()
		}
	}
}

// TestStreamChunkBoundaries checks that a stream produces exactly the same tokens as the in-memory parser when tokens, runes, numbers, strings with injections and whitespaces cross chunk boundaries.
// Readers that return less data than requested (like pipes and sockets) are checked too.
func TestStreamChunkBoundaries(t *testing.T) {
	parser := newBenchTokenizer()
	parser.DefineTokens(100, []string{"или"})
	parser.DefineStringToken(101, `<![CDATA[`, `]]>`)

	inputs := map[string]string{
		"tokens":     strings.Repeat("a, ", 50),
		"mixed":      string(benchSource(3)),
		"injections": strings.Repeat(`x = "value2 {{ value3 }} value4"; "{{ a }}{{ b }}"; `, 10) + `"tail {{ c`,
		"unicode":    strings.Repeat("привет или мир, функция\n", 10),
		"numbers":    strings.Repeat("1.5e10 2_000 .5 3E-2 1e+ 7. ", 10),
		"operators":  strings.Repeat("a <= b >= c == d != e <![CDATA[ socks ]]> ", 10),
		"whitespace": "  \n\t trailing ws \n\n  ",
	}
	readers := map[string]func(string) io.Reader{
		"full":    func(s string) io.Reader { return strings.NewReader(s) },
		"half":    func(s string) io.Reader { return iotest.HalfReader(strings.NewReader(s)) },
		"onebyte": func(s string) io.Reader { return iotest.OneByteReader(strings.NewReader(s)) },
	}
	sizes := []uint{4096}
	for size := uint(1); size <= 64; size++ {
		sizes = append(sizes, size)
	}

	for name, input := range inputs {
		expected := parser.ParseString(input)
		expectedTokens := collectTokens(expected)
		for readerName, reader := range readers {
			for _, size := range sizes {
				stream := parser.ParseStream(reader(input), size)
				require.Equalf(t, expectedTokens, collectTokens(stream), "input %s, reader %s, buffer size %d", name, readerName, size)
				require.Equalf(t, expected.GetParsedLength(), stream.GetParsedLength(), "input %s, reader %s, buffer size %d", name, readerName, size)
				stream.Close()
			}
		}
		expected.Close()
	}
}

// stutterReader returns (0, nil) several times before each byte of data.
type stutterReader struct {
	data   []byte
	empty  int
	misses int
}

func (r *stutterReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	if r.misses < r.empty || len(p) == 0 {
		r.misses++
		return 0, nil
	}
	r.misses = 0
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}

type noProgressReader struct{}

func (noProgressReader) Read([]byte) (int, error) {
	return 0, nil
}

// TestStreamReaders checks readers that return no data or an error.
func TestStreamReaders(t *testing.T) {
	parser := New()

	stream := parser.ParseStream(&stutterReader{data: []byte("one two three"), empty: 3}, 4)
	require.Equal(t, []string{"one", "two", "three"}, streamValues(stream))
	stream.Close()

	// a reader that never returns data must not hang the stream
	stream = parser.ParseStream(noProgressReader{}, 4)
	require.False(t, stream.IsValid())
	stream.Close()

	// the stream ends on a read error with the tokens parsed before it
	stream = parser.ParseStream(iotest.TimeoutReader(strings.NewReader("one two three")), 8)
	require.Equal(t, []string{"one", "two"}, streamValues(stream))
	stream.Close()

	stream = parser.ParseStream(iotest.DataErrReader(strings.NewReader("one two three")), 4)
	require.Equal(t, []string{"one", "two", "three"}, streamValues(stream))
	stream.Close()

	stream = parser.ParseStream(iotest.ErrReader(errors.New("broken")), 4)
	require.False(t, stream.IsValid())
	stream.Close()
}

func streamValues(stream *Stream) []string {
	var values []string
	for stream.IsValid() {
		values = append(values, stream.CurrentToken().ValueString())
		stream.GoNext()
	}
	return values
}

// goTo calls stream.GoTo and fails the test if it does not return.
func goTo(t *testing.T, stream *Stream, id int) *Stream {
	t.Helper()
	done := make(chan struct{})
	go func() {
		stream.GoTo(id)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("GoTo(%d) did not return", id)
	}
	return stream
}

// TestStreamGoTo GoTo moves the pointer to the token with the given id and doesn't move it if there is no such token.
func TestStreamGoTo(t *testing.T) {
	parser := New()
	stream := parser.ParseString("a b c") // ids 0..2
	defer stream.Close()

	require.Equal(t, 2, goTo(t, stream, 2).CurrentToken().ID())
	require.Equal(t, 0, goTo(t, stream, 0).CurrentToken().ID())
	require.Equal(t, 1, goTo(t, stream, 1).CurrentToken().ID())
	for _, id := range []int{3, 100, -1, -5} {
		require.Equalf(t, 1, goTo(t, stream, id).CurrentToken().ID(), "GoTo(%d)", id)
	}

	// beyond the end of the stream
	stream.GoNext().GoNext()
	require.False(t, stream.IsValid())
	for _, id := range []int{3, 100, -1, -5} {
		require.Falsef(t, goTo(t, stream, id).IsValid(), "GoTo(%d)", id)
	}
	require.Equal(t, 2, stream.GoPrev().CurrentToken().ID())
	stream.GoNext()
	require.Equal(t, 1, goTo(t, stream, 1).CurrentToken().ID())

	// before the beginning of the stream
	stream.GoPrev().GoPrev()
	require.False(t, stream.IsValid())
	for _, id := range []int{3, 100, -1, -5} {
		require.Falsef(t, goTo(t, stream, id).IsValid(), "GoTo(%d)", id)
	}
	require.Equal(t, 0, stream.GoNext().CurrentToken().ID())
	stream.GoPrev()
	require.Equal(t, 2, goTo(t, stream, 2).CurrentToken().ID())

	empty := parser.ParseString("")
	defer empty.Close()
	require.False(t, goTo(t, empty, 0).IsValid())
}

// TestStreamGoToInfinite GoTo parses the next chunks lazily and keeps all tokens when there is no such token.
func TestStreamGoToInfinite(t *testing.T) {
	parser := New()
	stream := parser.ParseStream(strings.NewReader(strings.Repeat("a ", 100)), 4) // ids 0..99
	defer stream.Close()

	require.Equal(t, 0, goTo(t, stream, 100).CurrentToken().ID())
	n := 0
	for ; stream.IsValid(); n++ {
		require.Equal(t, n, stream.CurrentToken().ID())
		stream.GoNext()
	}
	require.Equal(t, 100, n)
	require.Equal(t, 0, goTo(t, stream, 0).CurrentToken().ID())
	require.Equal(t, 99, goTo(t, stream, 99).CurrentToken().ID())
	require.Equal(t, 99, goTo(t, stream, 100).CurrentToken().ID())
}

// TestStreamGoToHistory GoTo frees the history behind the new position only, and stays if the token is before the beginning of the history.
func TestStreamGoToHistory(t *testing.T) {
	parser := New()
	stream := parser.ParseString("0 1 2 3 4 5 6 7 8 9").SetHistorySize(3)
	defer stream.Close()

	stream.GoNext()
	require.Equal(t, 1, goTo(t, stream, 100).CurrentToken().ID())
	require.Equal(t, 0, stream.HeadToken().ID())
	require.Equal(t, int64(0), stream.GoPrev().CurrentToken().ValueInt64())

	require.Equal(t, 9, goTo(t, stream, 9).CurrentToken().ID())
	require.Equal(t, 6, stream.HeadToken().ID())
	require.Equal(t, 4, stream.len)
	require.Equal(t, 9, goTo(t, stream, 0).CurrentToken().ID())
	require.Equal(t, 6, goTo(t, stream, 6).CurrentToken().ID())

	stream = parser.ParseStream(strings.NewReader(strings.Repeat("a ", 100)), 4).SetHistorySize(5)
	defer stream.Close()
	require.Equal(t, 50, goTo(t, stream, 50).CurrentToken().ID())
	require.Equal(t, 45, stream.HeadToken().ID())
	require.Equal(t, 50, goTo(t, stream, 44).CurrentToken().ID())
}

// TestStreamBounds GoNext and GoPrev stay out of the stream once they cross its bounds and step back with the opposite move.
func TestStreamBounds(t *testing.T) {
	parser := New()
	stream := parser.ParseString("a b c")
	defer stream.Close()

	stream.GoNext().GoNext().GoNext()
	require.False(t, stream.IsValid())
	require.False(t, stream.GoNext().IsValid())
	require.Equal(t, 2, stream.GoPrev().CurrentToken().ID())

	stream.GoTo(0).GoPrev()
	require.False(t, stream.IsValid())
	require.False(t, stream.GoPrev().IsValid())
	require.Equal(t, 0, stream.GoNext().CurrentToken().ID())

	// a look-ahead before the beginning of the stream keeps the position
	stream.GoPrev()
	require.True(t, stream.IsNextSequence(TokenKeyword, TokenKeyword, TokenKeyword))
	require.False(t, stream.IsAnyNextSequence([]TokenKey{TokenKeyword}, []TokenKey{TokenInteger}))
	require.False(t, stream.IsValid())
	require.Equal(t, 0, stream.GoNext().CurrentToken().ID())

	empty := parser.ParseString("")
	defer empty.Close()
	require.False(t, goTo(t, empty, 0).IsValid())
	require.False(t, empty.GoNext().IsValid())
	require.False(t, empty.GoPrev().IsValid())
}

func TestIssue7(t *testing.T) {
	p := New()
	buf := bytes.NewBuffer(nil)

	stream := p.ParseStream(buf, 4096)
	defer stream.Close()

	for stream.IsValid() {
		token := stream.CurrentToken()
		require.False(t, token.Is(TokenInteger))
		stream.GoNext()
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
