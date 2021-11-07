package tokenizer

import (
	"io"
	"unicode"
	"unicode/utf8"
)

const defaultBufferSize = 4096

// parsing is main parser
type parsing struct {
	t *Tokenizer
	pos      int
	line     int
	str      []byte
	buffer   []byte
	err      error
	reader   io.Reader
	token    *Token
	head     *Token
	ptr      *Token
	quoted   bool
	tail     []byte
	stopKeys []*TokenSettings
	n 		 int
	bufferSize int
}

func newParser(t *Tokenizer, str []byte) *parsing {
	return &parsing{
		t: t,
		str: str,
		line: 1,
		token: &Token{
			id: 0,
			line: 1,
		},
	}
}

func newInfParser(t *Tokenizer, reader io.Reader, bufferSize uint) *parsing {
	if bufferSize == 0 {
		bufferSize = defaultBufferSize
	}
	buffer := make([]byte, 0, bufferSize * 2)
	return &parsing{
		t: t,
		str: buffer,
		buffer: buffer,
		reader: reader,
		line: 1,
		bufferSize: int(bufferSize),
		token: &Token{
			id: 0,
			line: 1,
		},
	}
}

func (p *parsing) preload() {
	n, err := p.reader.Read(p.str)
	if n < p.bufferSize {
		p.str = p.str[:p.bufferSize+n]
		p.reader = nil
	}
	if err != nil {
		p.reader = nil
		if err != io.EOF {
			p.err = err
		}
	}
}

func (p *parsing) loadChunk() {
	if p.pos < p.bufferSize {
		return
	}
	copy(p.str, p.str[p.bufferSize:])
	p.pos -= p.bufferSize
	n, err := p.reader.Read(p.str[p.bufferSize:])

	if n < p.bufferSize {
		p.str = p.str[:p.bufferSize+n]
		p.reader = nil
	}

	if err != nil {
		p.reader = nil
		if err != io.EOF {
			p.err = err
		}
	}
}

// parse bytes (p.str) to tokens (p.list)
func (p *parsing) parse() {
	for {
		if p.stopKeys != nil {
			for _, t := range p.stopKeys {
				if p.ptr.key == t.Key {
					return
				}
			}
		}
		p.parseWhitespace()
		if p.pos >= len(p.str) {
			break
		}
		if p.parseToken() {
			continue
		}
		if p.pos >= len(p.str) {
			break
		}
		if p.parseKeyword() {
			continue
		}
		if p.pos >= len(p.str) {
			break
		}
		if p.parseNumber() {
			continue
		}
		if p.pos >= len(p.str) {
			break
		}
		if p.parseQuote() {
			continue
		}
		if p.pos >= len(p.str) {
			break
		}
		if p.t.flags & fStopOnUnknown != 0 {
			break
		}
		p.token.key = TokenUnknown
		p.token.value = p.str[p.pos:p.pos+1]
		p.token.offset = p.pos
		p.pos++
		p.emmitToken()
		if p.pos >= len(p.str) {
			break
		}
	}
	if len(p.token.indent) > 0 {
		p.tail = p.token.indent
	}
}

func (p *parsing) parseWhitespace() bool {
	var start = -1
	for p.pos < len(p.str) {
		var matched = false
		for _, ws := range p.t.wSpaces {
			if p.str[p.pos] == ws {
				if start == -1 {
					start = p.pos
				}
				matched = true
				break
			}
		}
		if !matched {
			break
		}
		if p.str[p.pos] == newLine {
			p.line++
		}
		p.pos++
	}
	if start != -1 {
		p.token.line = p.line
		p.token.indent = p.str[start:p.pos]
		return true
	} else {
		return false
	}
}

func (p *parsing) parseKeyword() bool {
	var start = -1
	for p.pos < len(p.str) {
		r, size := utf8.DecodeRune(p.str[p.pos:])
		if unicode.IsLetter(r) ||
			(p.t.flags & fAllowKeywordUnderscore != 0 && p.str[p.pos] == '_') ||
			(p.t.flags & fAllowNumberInKeyword != 0 && start != -1 && isNumberByte(p.str[p.pos])) {

			if start == -1 {
				start = p.pos
			}
			p.pos += size - 1 // rune may be more than 1 byte
		} else {
			break
		}
		p.pos++
	}
	if start != -1 {
		p.token.key = TokenKeyword
		p.token.value = p.str[start:p.pos]
		p.token.offset = start
		p.emmitToken()
		return true
	} else {
		return false
	}
}

const (
	stageCoefficient = iota + 1
	stageMantissa
	stagePower
)

func (p *parsing) parseNumber() bool {
	var start = -1
	var needNumber = true

	var stage uint8 = 0
	for p.pos < len(p.str) {
		if isNumberByte(p.str[p.pos]) {
			needNumber = false
			if start == -1 {
				if stage == 0 {
					stage = stageCoefficient
					start = p.pos
				}
			}
		} else if p.t.flags & fAllowNumberUnderscore != 0 && p.str[p.pos] == '_' {
			// todo checks double underscore
		} else if !needNumber && p.str[p.pos] == '.' {
			if stage != stageCoefficient {
				break
			}
			stage = stageMantissa
			needNumber = true
		} else if !needNumber && (p.str[p.pos] == 'e' || p.str[p.pos] == 'E') {
			if stage != stageMantissa && stage != stageCoefficient {
				break
			}
			ePowSign := false
			if p.pos + 1 < len(p.str) {
				switch p.str[p.pos + 1] {
				case '-', '+':
					ePowSign = true
					p.pos++
				}
			}
			needNumber = true
			if p.pos + 1 < len(p.str) && isNumberByte(p.str[p.pos + 1]) {
				stage = stagePower
			} else {
				if ePowSign { // rollback sign position
					p.pos--
				}
				break
			}
		} else {
			break
		}
		p.pos++
	}
	if stage == 0 {
		return false
	}
	p.token.value = p.str[start:p.pos]
	if stage == stageCoefficient {
		p.token.key = TokenInteger
		p.token.offset = start
	} else {
		p.token.key = TokenFloat
		p.token.offset = start
	}
	p.emmitToken()
	return true
}

func (p *parsing) match(r []byte, seek bool) bool {
	if r[0] == p.str[p.pos] {
		if len(r) > 1 {
			if p.isNext(r[1:], seek) {
				return true
			}
		} else {
			p.pos++
			return true
		}
	}
	return false
}

// parseQuote parses quoted string.
func (p *parsing) parseQuote() bool {
	var quote *QuoteSettings
	var start = p.pos
	for _, q := range p.t.quotes {
		if p.match(q.StartToken, true) {
			quote = q
			break
		}
	}
	if quote == nil {
		return false
	}
	p.token.key = TokenString
	p.token.offset = start
	p.token.string = quote
	escapes := false
	for p.pos < len(p.str) {
		if escapes {
			escapes = false
		} else if p.str[p.pos] == quote.EscapeSymbol {
			escapes = true
		} else if p.str[p.pos] == quote.EndToken[0] {
			if p.match(quote.EndToken, true) {
				break
			}
		} else if quote.Injects != nil {
			loop := true
			for _, inject := range quote.Injects {
				for _, token := range p.t.tokensMap[inject.StartKey] {
					if p.match(token.Token, true) {
						p.token.key = TokenStringFragment
						p.token.value = p.str[start:p.pos-len(token.Token)]
						p.emmitToken()
						p.token.key = token.Key
						p.token.value = token.Token
						p.token.offset = p.pos-len(token.Token)
						p.emmitToken()
						stopKeys := p.stopKeys // may be recursive quotes
						p.stopKeys = p.t.tokensMap[inject.EndKey]
						p.parse()
						p.stopKeys = stopKeys
						p.token.key = TokenStringFragment
						p.token.offset = p.pos
						p.token.string = quote
						start = p.pos
						loop = false
						break
					}
				}
				if !loop {
					break
				}
			}
		}
		if p.str[p.pos] == newLine {
			p.line++
		}
		p.pos++
	}
	p.token.value = p.str[start:p.pos]
	p.emmitToken()
	return true
}

// parseToken search any rune sequence from TokenSettings.
func (p *parsing) parseToken() bool {
	if p.pos < len(p.str) {
		start := p.pos
		for _, t := range p.t.tokens {
			// todo find longer token, build index for tokens
			if p.match(t.Token, true) {
				p.token.key = t.Key
				p.token.offset = start
				p.token.value = p.str[start:p.pos]
				p.emmitToken()
				return true
			}
		}
	}
	return false
}

func (p *parsing) isNext(s []byte, seek bool) bool {
	i := 1
	for _, c := range s {
		if p.pos + i < len(p.str) {
			if c != p.str[p.pos + i] {
				return false
			}
			i++
		} else {
			return false
		}
	}
	if seek {
		p.pos += i
	}
	return true
}

// emmitToken add new p.token to stream
func (p *parsing) emmitToken() {
	if p.ptr == nil {
		p.ptr = p.token
		p.head = p.ptr
	} else {
		p.ptr = p.ptr.addNext(p.token)
	}
	p.n++
	p.token = &Token{
		id: p.n,
		line: p.line,
	}
}



