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
	if len(p.str) == p.pos {
		if p.reader == nil || p.loadChunk() == 0 { // if it's not an infinite stream, or this is the end of the stream
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
		if p.t.stopOnUnknown {
			break
		}
		p.token.key = TokenUnknown
		p.token.value = p.str[p.pos : p.pos+1]
		p.token.offset = p.offset + p.pos
		p.emmitToken()
		if p.curr == 0 {
			break
		}
		p.next()
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
		if unicode.IsLetter(r) || runeExists(p.t.kwMajorSymbols, r) || (start != -1 && runeExists(p.t.kwMinorSymbols, r)) {
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

func (p *parsing) parseNumber() bool {
	var start = -1
	var end = -1
	var floatTraitPos = -1
	var hasPoint = false
	var hasNumber = false
	var hasExp = false

	for p.curr != 0 {
		if isNumberByte(p.curr) {
			if start == -1 {
				start = p.pos
			}
			end = p.pos
			hasNumber = true
		} else {
			nextByte := p.nextByte()
			if p.curr == '_' {
				if !hasNumber || (!p.t.allowNumberUnderscore || !isNumberByte(nextByte)) {
					break
				}
			} else if p.curr == '.' {
				if hasPoint {
					break
				} else if isNumberByte(nextByte) {
					if start == -1 { // floats can be started from a pointer
						start = p.pos
					}
				} else if !(nextByte == 'e' || nextByte == 'E' || nextByte == 0) {
					break
				}
				floatTraitPos = p.pos
				end = p.pos
				hasPoint = true
			} else if p.curr == 'e' || p.curr == 'E' {
				if !hasNumber || !(isNumberByte(nextByte) || nextByte == '-' || nextByte == '+') || hasExp {
					break
				}
				floatTraitPos = p.pos
				hasExp = true
				hasPoint = true
			} else if hasExp && (p.curr == '-' || p.curr == '+') {
				if isNumberByte(nextByte) {
					if start == -1 { // numbers can be started from a sign
						start = p.pos
					}
				} else {
					break
				}
			} else {
				break
			}
		}
		p.next()
	}
	if start == -1 {
		return false
	}
	end = end + 1
	p.pos = end
	p.token.value = p.str[start:end]
	if floatTraitPos == -1 || floatTraitPos > end-1 {
		p.token.key = TokenInteger
		p.token.offset = p.offset + start
	} else {
		p.token.key = TokenFloat
		p.token.offset = p.offset + start
	}
	p.emmitToken()
	return true
}

// ignore case equals a and b
func ignoreCaseEquals(a byte, b byte) bool {
	return upperCaseAlphabet(a) == upperCaseAlphabet(b)
}

func upperCaseAlphabet(c byte) byte {
	if c >= 'a' && c <= 'z' {
		c -= 32
	}
	return c
}

func isAlphabet(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// ignore case match compares next bytes from data with `r`
//
// copy match method add ignore case logic
func (p *parsing) ignoreCaseMatch(r []byte, seek bool) bool {
	if ignoreCaseEquals(r[0], p.curr) {
		if len(r) > 1 {
			if p.ensureBytes(len(r) - 1) {
				var i = 1
				for ; i < len(r); i++ {
					if !ignoreCaseEquals(r[i], p.str[p.pos+i]) {
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

// match compares next bytes from data with `r`
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
		if isAlphabet(p.curr) {
			upCaseByte := upperCaseAlphabet(p.curr)
			c := p.t.icIndex[upCaseByte]
			toks = append(toks, c...)
		}
		// toks := p.t.index[p.curr]
		if toks != nil {
			start := p.pos
			for _, t := range toks {

				var matchFn func(r []byte, seek bool) bool
				if t.IgnoreCase {
					matchFn = p.ignoreCaseMatch
				} else {
					matchFn = p.match
				}

				if matchFn(t.Token, true) {
					// alone split patch
					if t.Alone && len(p.str) > start+len(t.Token) {
						nt := p.str[start+len(t.Token)]
						if nt >= '0' && nt <= '9' || nt >= 'a' && nt <= 'z' || nt >= 'A' && nt <= 'Z' || nt == '_' {
							// rollback pos
							p.pos = p.pos - (len(t.Token))
							p.curr = p.str[p.pos]
							continue
						}
					}
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
