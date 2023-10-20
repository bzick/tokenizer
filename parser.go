package tokenizer

import (
	"io"
	"unicode"
	"unicode/utf8"
)

// DefaultChunkSize default chunk size for reader.
const DefaultChunkSize = 4096

// parsing is main parser
type parsing struct {
	t         *Tokenizer
	curr      byte
	pos       int
	line      int
	str       []byte
	err       error
	reader    io.Reader
	token     *Token
	head      *Token
	ptr       *Token
	tail      []byte
	stopKeys  []*tokenRef
	n         int // tokens id generator
	chunkSize int // chunks size for infinite buffer
	offset    int
	resume    bool
	parsed    int
}

// newParser creates new parser for string
func newParser(t *Tokenizer, str []byte) *parsing {
	tok := t.allocToken()
	tok.line = 1
	return &parsing{
		t:     t,
		str:   str,
		line:  1,
		token: tok,
	}
}

func newInfParser(t *Tokenizer, reader io.Reader, bufferSize uint) *parsing {
	if bufferSize == 0 {
		bufferSize = DefaultChunkSize
	}
	buffer := make([]byte, bufferSize)
	tok := t.allocToken()
	tok.line = 1
	return &parsing{
		t:         t,
		str:       buffer,
		reader:    reader,
		line:      1,
		chunkSize: int(bufferSize),
		token:     tok,
	}
}

func (p *parsing) prev() {
	if p.pos > 0 {
		p.pos--
		p.curr = p.str[p.pos]
	}
}

func (p *parsing) ensureBytes(n int) bool {
	if p.pos+n >= len(p.str) {
		if p.reader != nil {
			p.loadChunk()
			if p.pos+n < len(p.str) {
				return true
			}
		}
		return false
	}
	return true
}

func (p *parsing) next() {
	p.pos++
	if p.pos >= len(p.str) {
		if p.reader == nil || p.loadChunk() == 0 {
			p.curr = 0
			return
		}
	}
	p.curr = p.str[p.pos]
}

func (p *parsing) nextByte() byte {
	if p.ensureBytes(1) {
		return p.str[p.pos+1]
	}
	return 0
}

func (p *parsing) slice(from, to int) []byte {
	if to < len(p.str) {
		return p.str[from:to]
	}
	return p.str[from:]
}

func (p *parsing) preload() {
	n, err := p.reader.Read(p.str)
	if n < p.chunkSize {
		p.str = p.str[:n]
		p.reader = nil
	}
	if err != nil {
		p.reader = nil
		if err != io.EOF {
			p.err = err
		}
	}
}

func (p *parsing) loadChunk() int {
	// chunk size = new chunk size + size of tail of prev chunk
	chunk := make([]byte, len(p.str)+p.chunkSize)
	copy(chunk, p.str)
	n, err := p.reader.Read(chunk[len(p.str):])

	if n < p.chunkSize {
		p.str = chunk[:len(p.str)+n]
		p.reader = nil
	} else {
		p.str = chunk
	}

	if err != nil {
		p.reader = nil
		if err != io.EOF {
			p.err = err
		}
	}
	p.resume = false
	return n
}

// checkPoint reset internal values for next chunk of data
func (p *parsing) checkPoint() bool {
	if p.pos > 0 {
		p.parsed += p.pos
		p.str = p.str[p.pos:]
		p.offset += p.pos
		p.pos = 0
		if len(p.str) == 0 {
			p.curr = 0
		}
	}
	return p.resume
}

// parse bytes (p.str) to tokens and append them to the end if stream of tokens.
func (p *parsing) parse() {
	if len(p.str) == 0 {
		if p.reader == nil || p.loadChunk() == 0 { // if it's not infinite stream or this is the end of stream
			return
		}
	}
	p.curr = p.str[p.pos]
	p.resume = true
	for p.checkPoint() {
		if p.stopKeys != nil {
			for _, t := range p.stopKeys {
				if p.ptr.key == t.Key {
					return
				}
			}
		}
		p.parseWhitespace()
		if p.curr == 0 {
			break
		}
		if p.parseToken() {
			continue
		}
		if p.curr == 0 {
			break
		}
		if p.parseKeyword() {
			continue
		}
		if p.curr == 0 {
			break
		}
		if p.parseNumber() {
			continue
		}
		if p.curr == 0 {
			break
		}
		if p.parseQuote() {
			continue
		}
		if p.curr == 0 {
			break
		}
		if p.t.flags&fStopOnUnknown != 0 {
			break
		}
		p.token.key = TokenUnknown
		p.token.value = p.str[p.pos : p.pos+1]
		p.token.offset = p.offset + p.pos
		p.next()
		p.emmitToken()
		if p.curr == 0 {
			break
		}
	}
	if len(p.token.indent) > 0 {
		p.tail = p.token.indent
	}
}

func (p *parsing) parseWhitespace() bool {
	var start = -1
	for p.curr != 0 {
		var matched = false
		for _, ws := range p.t.wSpaces {
			if p.curr == ws {
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
		if p.curr == newLine {
			p.line++
		}
		p.next()
	}
	if start != -1 {
		p.token.line = p.line
		p.token.indent = p.str[start:p.pos]
		return true
	}
	return false
}

func (p *parsing) parseKeyword() bool {
	var start = -1
	for p.curr != 0 {
		var r rune
		var size int
		p.ensureBytes(4)
		r, size = utf8.DecodeRune(p.slice(p.pos, p.pos+4))
		if unicode.IsLetter(r) ||
			(p.t.flags&fAllowKeywordUnderscore != 0 && p.curr == '_') ||
			(p.t.flags&fAllowNumberInKeyword != 0 && start != -1 && isNumberByte(p.curr)) ||
			(p.t.flags&fAllowAtInKeyword != 0 && p.curr == '@') ||
			(p.t.flags&fAllowDotInKeyword != 0 && p.curr == '.') {

			if start == -1 {
				start = p.pos
			}
			p.pos += size - 1 // rune may be more than 1 byte
		} else {
			break
		}
		p.next()
	}
	if start != -1 {
		p.token.key = TokenKeyword
		p.token.value = p.str[start:p.pos]
		p.token.offset = p.offset + start
		p.emmitToken()
		return true
	}
	return false
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
	for p.curr != 0 {
		if isNumberByte(p.curr) {
			needNumber = false
			if start == -1 {
				if stage == 0 {
					stage = stageCoefficient
					start = p.pos
				}
			}
		} else if p.t.flags&fAllowNumberUnderscore != 0 && p.curr == '_' {
			if stage != stageCoefficient {
				break
			}
			// todo checks double underscore
		} else if !needNumber && p.curr == '.' {
			if stage != stageCoefficient {
				break
			}
			stage = stageMantissa
			needNumber = true
		} else if !needNumber && (p.curr == 'e' || p.curr == 'E') {
			if stage != stageMantissa && stage != stageCoefficient {
				break
			}
			ePowSign := false
			switch p.nextByte() {
			case '-', '+':
				ePowSign = true
				p.next()
			}
			needNumber = true
			if isNumberByte(p.nextByte()) {
				stage = stagePower
			} else {
				if ePowSign { // rollback sign position
					p.prev()
				}
				break
			}
		} else {
			break
		}
		p.next()
	}
	if stage == 0 {
		return false
	}
	p.token.value = p.str[start:p.pos]
	if stage == stageCoefficient {
		p.token.key = TokenInteger
		p.token.offset = p.offset + start
	} else {
		p.token.key = TokenFloat
		p.token.offset = p.offset + start
	}
	p.emmitToken()
	return true
}

// match compare next bytes from data with `r`
func (p *parsing) match(r []byte, seek bool) bool {
	if r[0] == p.curr {
		if len(r) > 1 {
			if p.ensureBytes(len(r) - 1) {
				var i = 1
				for ; i < len(r); i++ {
					if r[i] != p.str[p.pos+i] {
						return false
					}
				}
				if seek {
					p.pos += i - 1
					p.next()
				}
				return true
			}
			return false
		}
		if seek {
			p.next()
		}
		return true
	}
	return false
}

// parseQuote parses quoted string.
func (p *parsing) parseQuote() bool {
	var quote *StringSettings
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
	p.token.offset = p.offset + start
	p.token.string = quote
	escapes := false
	for p.curr != 0 {
		if escapes {
			escapes = false
		} else if p.curr == quote.EscapeSymbol {
			escapes = true
		} else if p.match(quote.EndToken, true) {
			break
		} else if quote.Injects != nil {
			loop := true
			for _, inject := range quote.Injects {
				for _, token := range p.t.tokens[inject.StartKey] {
					if p.match(token.Token, true) {
						p.token.key = TokenStringFragment
						p.token.value = p.str[start : p.pos-len(token.Token)]
						p.emmitToken()
						p.token.key = token.Key
						p.token.value = token.Token
						p.token.offset = p.offset + p.pos - len(token.Token)
						p.emmitToken()
						stopKeys := p.stopKeys // may be recursive quotes
						p.stopKeys = p.t.tokens[inject.EndKey]
						p.parse()
						p.stopKeys = stopKeys
						p.token.key = TokenStringFragment
						p.token.offset = p.offset + p.pos
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
		if p.curr == newLine {
			p.line++
		}
		p.next()
	}
	p.token.value = p.str[start:p.pos]
	p.emmitToken()
	return true
}

// parseToken search any rune sequence from tokenItem.
func (p *parsing) parseToken() bool {
	if p.curr != 0 {
		toks := p.t.index[p.curr]
		if toks != nil {
			start := p.pos
			for _, t := range toks {
				if p.match(t.Token, true) {
					p.token.key = t.Key
					p.token.offset = p.offset + start
					p.token.value = t.Token
					p.emmitToken()
					return true
				}
			}
		}
	}
	return false
}

// emmitToken add new p.token to stream
func (p *parsing) emmitToken() {
	if p.ptr == nil {
		p.ptr = p.token
		p.head = p.ptr
	} else {
		p.ptr.addNext(p.token)
		p.ptr = p.token
	}
	p.n++
	p.token = p.t.allocToken()
	p.token.id = p.n
	p.token.line = p.line
}
