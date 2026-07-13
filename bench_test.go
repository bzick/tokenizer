package tokenizer

import (
	"bytes"
	"strings"
	"testing"
)

const (
	bTokenPlus = iota + 1
	bTokenMinus
	bTokenMul
	bTokenDiv
	bTokenAssign
	bTokenEq
	bTokenNeq
	bTokenLt
	bTokenGt
	bTokenLte
	bTokenGte
	bTokenOpen
	bTokenClose
	bTokenComma
	bTokenSemicolon
	bTokenDQuote
	bTokenInjStart
	bTokenInjEnd
)

func newBenchTokenizer() *Tokenizer {
	t := New()
	t.AllowKeywordSymbols(Underscore, Numbers)
	t.AllowNumberUnderscore()
	t.DefineTokens(bTokenPlus, []string{"+"})
	t.DefineTokens(bTokenMinus, []string{"-"})
	t.DefineTokens(bTokenMul, []string{"*"})
	t.DefineTokens(bTokenDiv, []string{"/"})
	t.DefineTokens(bTokenAssign, []string{"="})
	t.DefineTokens(bTokenEq, []string{"=="})
	t.DefineTokens(bTokenNeq, []string{"!="})
	t.DefineTokens(bTokenLt, []string{"<"})
	t.DefineTokens(bTokenGt, []string{">"})
	t.DefineTokens(bTokenLte, []string{"<="})
	t.DefineTokens(bTokenGte, []string{">="})
	t.DefineTokens(bTokenOpen, []string{"("})
	t.DefineTokens(bTokenClose, []string{")"})
	t.DefineTokens(bTokenComma, []string{","})
	t.DefineTokens(bTokenSemicolon, []string{";"})
	t.DefineTokens(bTokenInjStart, []string{"{{"})
	t.DefineTokens(bTokenInjEnd, []string{"}}"})
	t.DefineStringToken(bTokenDQuote, `"`, `"`).
		SetEscapeSymbol('\\').
		AddSpecialStrings(DefaultSpecialString).
		AddInjection(bTokenInjStart, bTokenInjEnd)
	return t
}

// mixed source: keywords, numbers, operators, strings
func benchSource(repeat int) []byte {
	unit := `user_id = 42; total_amount = 1_000.5e3;
if (user_name == "hello \"world\" {{ variable_name }} tail") result = total + 15.75;
while (counter_value <= 9000) counter_value = counter_value + 1;
send(user_id, "текст сообщения", 3.14159, offset_9);
`
	return bytes.Repeat([]byte(unit), repeat)
}

func BenchmarkParseBytesMixed(b *testing.B) {
	t := newBenchTokenizer()
	src := benchSource(50) // ~10KB
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseBytes(src)
		st.Close()
	}
}

func BenchmarkParseKeywordsOnly(b *testing.B) {
	t := newBenchTokenizer()
	src := bytes.Repeat([]byte("alpha bravo charlie_delta echo_1 foxtrot golf hotel india juliett kilo "), 150)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseBytes(src)
		st.Close()
	}
}

func BenchmarkParseNumbersOnly(b *testing.B) {
	t := newBenchTokenizer()
	src := bytes.Repeat([]byte("123 45.67 1e9 1.5E-3 1_000_000 42 987654321 0.001 "), 200)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseBytes(src)
		st.Close()
	}
}

func BenchmarkParseOperatorsOnly(b *testing.B) {
	t := newBenchTokenizer()
	src := bytes.Repeat([]byte("+ - * / == != <= >= ( ) , ; = < > "), 300)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseBytes(src)
		st.Close()
	}
}

func BenchmarkParseStringsOnly(b *testing.B) {
	t := newBenchTokenizer()
	src := bytes.Repeat([]byte(`"simple string" "with \"escape\"" "with {{ inject }} injection" `), 150)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseBytes(src)
		st.Close()
	}
}

func BenchmarkParseUnicodeKeywords(b *testing.B) {
	t := newBenchTokenizer()
	src := bytes.Repeat([]byte("привет мир функция переменная значение цикл условие "), 150)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseBytes(src)
		st.Close()
	}
}

func BenchmarkParseStream(b *testing.B) {
	t := newBenchTokenizer()
	src := benchSource(500) // ~100KB
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseStream(bytes.NewReader(src), 4096).SetHistorySize(10)
		for st.IsValid() {
			st.GoNext()
		}
		st.Close()
	}
}

func BenchmarkParseStreamNoHistory(b *testing.B) {
	t := newBenchTokenizer()
	src := benchSource(500)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := t.ParseStream(bytes.NewReader(src), 4096)
		for st.IsValid() {
			st.GoNext()
		}
		st.Close()
	}
}

func BenchmarkValueUnescaped(b *testing.B) {
	t := newBenchTokenizer()
	st := t.ParseString(`"one \"two\" \t three \n four"`)
	defer st.Close()
	tok := st.CurrentToken()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tok.ValueUnescaped()
	}
}

var sinkStr string

func BenchmarkStreamString(b *testing.B) {
	t := newBenchTokenizer()
	st := t.ParseBytes(benchSource(10))
	defer st.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = st.String()
	}
}

func init() {
	_ = strings.Repeat // keep import
}
