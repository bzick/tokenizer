package tokenizer

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTokenize(t *testing.T) {
	type item struct {
		str   string
		token Token
	}
	tokenizer := New()
	condTokenKey := 10
	wordTokenKey := 11
	tokenizer.DefineTokens(condTokenKey, []string{">=", "<=", "==", ">", "<"})
	tokenizer.DefineTokens(wordTokenKey, []string{"or", "или"})
	quote := tokenizer.AddString(`"`, `"`).SetEscapeSymbol('\\')
	data := []item{
		{"one", Token{key: TokenKeyword}},
		{"два", Token{key: TokenKeyword}},
		{"1", Token{key: TokenInteger}},
		{"2.3", Token{key: TokenFloat}},
		{"2.", Token{key: TokenFloat}},
		{"2.3e4", Token{key: TokenFloat}},
		{"2.3e-4", Token{key: TokenFloat}},
		{"2.3E+4", Token{key: TokenFloat}},
		{"2e4", Token{key: TokenFloat}},
		{"\"one\"", Token{key: TokenString, string: quote}},
		{"\"one two\"", Token{key: TokenString, string: quote}},
		{"\"два три\"", Token{key: TokenString, string: quote}},
		{"\"one\\\" two\"", Token{key: TokenString, string: quote}},
		{"\"\"", Token{key: TokenString, string: quote}},
		{">=", Token{key: condTokenKey}},
		{"<", Token{key: condTokenKey}},
		{"=", Token{key: TokenUnknown}},
		{"or", Token{key: wordTokenKey}},
		{"или", Token{key: wordTokenKey}},
	}

	for _, v := range data {
		stream := tokenizer.ParseBytes([]byte(v.str))
		expected := &v.token
		expected.value = []byte(v.str)
		actual := &Token{
			value:  stream.current.value,
			key:    stream.current.key,
			string: stream.current.string,
		}
		require.Equalf(t, expected, actual, "parse %s: %s", v.str, stream.current)
	}
}

func TestTokenizeEdgeCases(t *testing.T) {
	type item struct {
		str    string
		tokens []Token
	}
	tokenizer := New()

	data1 := []item{
		{"one1", []Token{
			{key: TokenKeyword, value: s2b("one"), offset: 0, line: 1, id: 0},
			{key: TokenInteger, value: s2b("1"), offset: 3, line: 1, id: 1},
		}},
		{"one_two", []Token{
			{key: TokenKeyword, value: s2b("one"), offset: 0, line: 1, id: 0},
			{key: TokenUnknown, value: s2b("_"), offset: 3, line: 1, id: 1},
			{key: TokenKeyword, value: s2b("two"), offset: 4, line: 1, id: 2},
		}},
		{"one_1", []Token{
			{key: TokenKeyword, value: s2b("one"), offset: 0, line: 1, id: 0},
			{key: TokenUnknown, value: s2b("_"), offset: 3, line: 1, id: 1},
			{key: TokenInteger, value: s2b("1"), offset: 4, line: 1, id: 2},
		}},
	}
	data2 := []item{
		{"one1", []Token{
			{key: TokenKeyword, value: s2b("one1"), offset: 0, line: 1, id: 0},
		}},
		{"one_two", []Token{
			{key: TokenKeyword, value: s2b("one_two"), offset: 0, line: 1, id: 0},
		}},
		{"one_1", []Token{
			{key: TokenKeyword, value: s2b("one_1"), offset: 0, line: 1, id: 0},
		}},
	}

	for _, v := range data1 {
		stream := tokenizer.ParseString(v.str)
		require.Equalf(t, v.tokens, stream.GetSnippet(10, 10), "parse data1 %s: %s", v.str, stream)
	}

	tokenizer.AllowNumbersInKeyword().AllowKeywordUnderscore()

	for _, v := range data2 {
		stream := tokenizer.ParseBytes([]byte(v.str))
		require.Equalf(t, v.tokens, stream.GetSnippet(10, 10), "parse data2 %s: %s", v.str, stream)
	}
}

func TestTokenizeComplex(t *testing.T) {
	tokenizer := New()
	compareTokenKey := 10
	condTokenKey := 11
	tokenizer.AllowKeywordUnderscore()
	tokenizer.DefineTokens(compareTokenKey, []string{">=", "<=", "==", ">", "<", "="})
	tokenizer.DefineTokens(condTokenKey, []string{"and", "or"})
	quote := tokenizer.AddString(`"`, `"`).SetEscapeSymbol('\\')
	quote2 := tokenizer.AddString("'", "'").SetEscapeSymbol('\\')

	str := "modified >\t\"2021-10-06 12:30:44\" and \nbytes_in <= 100 or user_agent='curl'"
	stream := tokenizer.ParseString(str)

	require.Equalf(t, []Token{
		{
			id:     0,
			key:    TokenKeyword,
			value:  []byte("modified"),
			offset: 0,
			line:   1,
		},
		{
			id:     1,
			key:    compareTokenKey,
			value:  []byte(">"),
			indent: []byte(" "),
			offset: 9,
			line:   1,
		},
		{
			id:     2,
			key:    TokenString,
			value:  []byte("\"2021-10-06 12:30:44\""),
			indent: []byte("\t"),
			offset: 11,
			line:   1,
			string: quote,
		},
		{
			id:     3,
			key:    condTokenKey,
			value:  []byte("and"),
			indent: []byte(" "),
			line:   1,
			offset: 33,
		},
		{
			id:     4,
			key:    TokenKeyword,
			value:  []byte("bytes_in"),
			indent: []byte(" \n"),
			offset: 38,
			line:   2,
		},
		{
			id:     5,
			key:    compareTokenKey,
			value:  []byte("<="),
			indent: []byte(" "),
			offset: 47,
			line:   2,
		},
		{
			id:     6,
			key:    TokenInteger,
			value:  []byte("100"),
			indent: []byte(" "),
			offset: 50,
			line:   2,
		},
		{
			id:     7,
			key:    condTokenKey,
			value:  []byte("or"),
			indent: []byte(" "),
			offset: 54,
			line:   2,
		},
		{
			id:     8,
			key:    TokenKeyword,
			value:  []byte("user_agent"),
			indent: []byte(" "),
			offset: 57,
			line:   2,
		},
		{
			id:     9,
			key:    compareTokenKey,
			value:  []byte("="),
			indent: nil,
			offset: 67,
			line:   2,
		},
		{
			id:     10,
			key:    TokenString,
			value:  []byte("'curl'"),
			indent: nil,
			offset: 68,
			string: quote2,
			line:   2,
		},
	}, stream.GetSnippet(10, 100), "parsed %s as \n%s", str, stream)
}

func TestTokenizeInject(t *testing.T) {
	tokenizer := New()
	startQuoteVarToken := 10
	endQuoteVarToken := 11
	tokenizer.DefineTokens(startQuoteVarToken, []string{"{{"})
	tokenizer.DefineTokens(endQuoteVarToken, []string{"}}"})

	quote := tokenizer.AddString(`"`, `"`).
		SetEscapeSymbol('\\').
		AddInjection(startQuoteVarToken, endQuoteVarToken)

	str := `"one {{ two }} three"`
	stream := tokenizer.ParseString(str)

	require.Equalf(t, []Token{
		{
			id:     0,
			key:    TokenStringFragment,
			value:  []byte("\"one "),
			offset: 0,
			string: quote,
			line:   1,
		},
		{
			id:     1,
			key:    startQuoteVarToken,
			value:  []byte("{{"),
			offset: 5,
			indent: nil,
			line:   1,
		},
		{
			id:     2,
			key:    TokenKeyword,
			value:  []byte("two"),
			offset: 8,
			indent: []byte(" "),
			line:   1,
		},
		{
			id:     3,
			key:    endQuoteVarToken,
			value:  []byte("}}"),
			offset: 12,
			indent: []byte(" "),
			line:   1,
		},
		{
			id:     4,
			key:    TokenStringFragment,
			value:  []byte(" three\""),
			offset: 14,
			indent: nil,
			string: quote,
			line:   1,
		},
	}, stream.GetSnippet(10, 10), "parsed %s as %s", str, stream)
}
