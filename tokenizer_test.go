package tokenizer

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTokenize(t *testing.T) {
	type item struct {
		value interface{}
		token Token
	}
	tokenizer := New()
	condTokenKey := TokenKey(10)
	wordTokenKey := TokenKey(11)
	dquoteKey := TokenKey(14)
	tokenizer.AllowNumberUnderscore()
	tokenizer.DefineTokens(condTokenKey, []string{">=", "<=", "==", ">", "<"})
	tokenizer.DefineTokens(wordTokenKey, []string{"or", "или"})
	tokenizer.SetWhiteSpaces([]byte{' ', '\t', '\n'})
	quote := tokenizer.DefineStringToken(dquoteKey, `"`, `"`).
		SetEscapeSymbol('\\').
		AddSpecialStrings([]string{`"`})

	t.Run("any", func(t *testing.T) {
		data := []item{
			{"one", Token{key: TokenKeyword, value: []byte("one")}},
			{"два", Token{key: TokenKeyword, value: []byte("one")}},
			{">=", Token{key: condTokenKey, value: []byte(">=")}},
			{"<", Token{key: condTokenKey, value: []byte("<")}},
			{"=", Token{key: TokenUnknown, value: []byte("=")}},
			{"or", Token{key: wordTokenKey, value: []byte("or")}},
			{"или", Token{key: wordTokenKey, value: []byte("или")}},
		}

		for _, v := range data {
			stream := tokenizer.ParseBytes(v.token.value)
			require.Equal(t, v.token.Value(), stream.CurrentToken().Value())
			require.Equal(t, v.token.Key(), stream.CurrentToken().Key())
			require.Equal(t, v.token.StringSettings(), stream.CurrentToken().StringSettings())
		}
	})

	t.Run("integers", func(t *testing.T) {
		integers := []item{
			{int64(1), Token{key: TokenInteger, value: []byte("1")}},
			{int64(123456), Token{key: TokenInteger, value: []byte("123456")}},
			{int64(123456), Token{key: TokenInteger, value: []byte("123_456")}},
		}
		for _, v := range integers {
			stream := tokenizer.ParseBytes(v.token.value)
			require.Equal(t, v.token.Value(), stream.CurrentToken().Value())
			require.Equal(t, v.token.Key(), stream.CurrentToken().Key())
			require.Equal(t, v.token.StringSettings(), stream.CurrentToken().StringSettings())
			require.Equal(t, v.value, stream.CurrentToken().ValueInt64(), "value %s -> %d: %s", v.token.value, v.value, stream.CurrentToken().Value())
		}
	})

	t.Run("floats", func(t *testing.T) {
		floats := []item{
			{2.3, Token{key: TokenFloat, value: []byte("2.3")}},
			{2.0, Token{key: TokenFloat, value: []byte("2.")}},
			{0.2, Token{key: TokenFloat, value: []byte(".2")}},
			{2.3e4, Token{key: TokenFloat, value: []byte("2.3e4")}},
			{2.3e-4, Token{key: TokenFloat, value: []byte("2.3e-4")}},
			{2.3e+4, Token{key: TokenFloat, value: []byte("2.3E+4")}},
			{2e4, Token{key: TokenFloat, value: []byte("2e4")}},
		}
		for _, v := range floats {
			stream := tokenizer.ParseBytes(v.token.value)
			require.Equal(t, v.token.Value(), stream.CurrentToken().Value())
			require.Equal(t, v.token.Key(), stream.CurrentToken().Key())
			require.Equal(t, v.token.StringSettings(), stream.CurrentToken().StringSettings())
			require.Equal(t, v.value, stream.CurrentToken().ValueFloat64())
		}
	})

	t.Run("framed", func(t *testing.T) {
		framed := []item{
			{"one", Token{key: TokenString, string: quote, value: []byte("\"one\"")}},
			{"one two", Token{key: TokenString, string: quote, value: []byte("\"one two\"")}},
			{"два три", Token{key: TokenString, string: quote, value: []byte("\"два три\"")}},
			{"one\" two", Token{key: TokenString, string: quote, value: []byte(`"one\" two"`)}},
			{"", Token{key: TokenString, string: quote, value: []byte("\"\"")}},
		}
		for _, v := range framed {
			stream := tokenizer.ParseBytes(v.token.value)
			require.Equal(t, v.token.Value(), stream.CurrentToken().Value())
			require.Equal(t, v.token.Key(), stream.CurrentToken().Key())
			require.Equal(t, v.token.StringSettings(), stream.CurrentToken().StringSettings())
			require.Equal(t, v.value, stream.CurrentToken().ValueUnescapedString(), "value %s -> %s: %s", v.token.value, v.value, stream.CurrentToken().Value())
		}
	})
}

func TestTokenizeEdgeCases(t *testing.T) {
	type item struct {
		str    string
		tokens []Token
	}
	tokenizer := New()

	t.Run("cases1", func(t *testing.T) {
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
			{"1..2", []Token{ // https://github.com/bzick/tokenizer/issues/11
				{key: TokenInteger, value: s2b("1"), offset: 0, line: 1, id: 0},
				{key: TokenUnknown, value: s2b("."), offset: 1, line: 1, id: 1},
				{key: TokenFloat, value: s2b(".2"), offset: 2, line: 1, id: 2},
			}},
			{"1ee2", []Token{
				{key: TokenInteger, value: s2b("1"), offset: 0, line: 1, id: 0},
				{key: TokenKeyword, value: s2b("ee"), offset: 1, line: 1, id: 1},
				{key: TokenInteger, value: s2b("2"), offset: 3, line: 1, id: 2},
			}},
			{"1e-s", []Token{
				{key: TokenInteger, value: s2b("1"), offset: 0, line: 1, id: 0},
				{key: TokenKeyword, value: s2b("e"), offset: 1, line: 1, id: 1},
				{key: TokenUnknown, value: s2b("-"), offset: 2, line: 1, id: 2},
				{key: TokenKeyword, value: s2b("s"), offset: 3, line: 1, id: 3},
			}},
			{".1.2", []Token{
				{key: TokenFloat, value: s2b(".1"), offset: 0, line: 1, id: 0},
				{key: TokenFloat, value: s2b(".2"), offset: 2, line: 1, id: 1},
			}},
			{"a]", []Token{ // https://github.com/bzick/tokenizer/issues/9
				{key: TokenKeyword, value: s2b("a"), offset: 0, line: 1, id: 0},
				{key: TokenUnknown, value: s2b("]"), offset: 1, line: 1, id: 1},
			}},
		}
		for _, v := range data1 {
			stream := tokenizer.ParseString(v.str)
			require.Equalf(t, v.tokens, stream.GetSnippet(10, 10), "parse data1 %s: %s", v.str, stream)
		}
	})
	t.Run("case2", func(t *testing.T) {
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

		tokenizer.AllowKeywordSymbols(Underscore, Numbers)

		for _, v := range data2 {
			stream := tokenizer.ParseBytes([]byte(v.str))
			require.Equalf(t, v.tokens, stream.GetSnippet(10, 10), "parse data2 %s: %s", v.str, stream)
		}
	})
}

func TestTokenizeComplex(t *testing.T) {
	tokenizer := New()
	compareTokenKey := TokenKey(10)
	condTokenKey := TokenKey(11)
	quoteTokenKey := TokenKey(14)
	tokenizer.AllowKeywordSymbols(Underscore, nil)
	tokenizer.DefineTokens(compareTokenKey, []string{">=", "<=", "==", ">", "<", "="})
	tokenizer.DefineTokens(condTokenKey, []string{"and", "or"})
	quote := tokenizer.DefineStringToken(quoteTokenKey, `"`, `"`).SetEscapeSymbol('\\')
	quote2 := tokenizer.DefineStringToken(quoteTokenKey, "'", "'").SetEscapeSymbol('\\')

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
	startQuoteVarToken := TokenKey(10)
	endQuoteVarToken := TokenKey(11)
	quoteTokenKey := TokenKey(14)
	tokenizer.DefineTokens(startQuoteVarToken, []string{"{{"})
	tokenizer.DefineTokens(endQuoteVarToken, []string{"}}"})

	quote := tokenizer.DefineStringToken(quoteTokenKey, `"`, `"`).
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
