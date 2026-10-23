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
	tok := t.allocToken()
	tok.line = 1
	return &parsing{
		t:         t,
		str:       make([]byte, 0, bufferSize), // the first chunk is read by the first parse() call
		reader:    reader,
		line:      1,
		chunkSize: int(bufferSize),
		token:     tok,
	}
}

// ensureBytes makes sure that the byte at p.pos+n is loaded.
// A reader may return less data than requested, so chunks are loaded until there is enough data or the stream ends.
func (p *parsing) ensureBytes(n int) bool {
	for p.pos+n >= len(p.str) {
		if p.reader == nil {
			return false
		}
		p.loadChunk()
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

// maxEmptyReads is the number of consecutive empty reads (0, nil) after which the reader is treated as broken, like in bufio.
const maxEmptyReads = 100

// loadChunk reads the next portion of data from the reader and appends it to p.str.
// A short read is not the end of the data: the stream ends only when the reader returns an error (io.EOF included).
// Returns the number of loaded bytes; zero means the end of the stream.
func (p *parsing) loadChunk() int {
	if len(p.str) == cap(p.str) {
		// chunk size = new chunk size + size of tail of prev chunk.
		// The old buffer is not reused because the parsed tokens point into it.
		chunk := make([]byte, len(p.str), len(p.str)+p.chunkSize)
		copy(chunk, p.str)
		p.str = chunk
	}
	p.resume = false
	for i := 0; i < maxEmptyReads; i++ {
		n, err := p.reader.Read(p.str[len(p.str):cap(p.str)])
		p.str = p.str[:len(p.str)+n]
		if err != nil {
			p.reader = nil
			if err != io.EOF {
				p.err = err
			}
			return n
		}
		if n > 0 {
			return n
		}
	}
	p.reader = nil
	p.err = io.ErrNoProgress
	return 0
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
	// a loaded chunk stops parsing after the current token (lazy parsing), but an injection (p.stopKeys) is parsed up to its end token.
	for p.checkPoint() || p.stopKeys != nil {
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
	for p.curr != 0 && p.t.wsTable[p.curr] {
		if start == -1 {
			start = p.pos
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
		// p.curr may be stale after parseNumber rolls back p.pos, so read the byte at p.pos,
		// like utf8.DecodeRune below does.
		if c := p.str[p.pos]; c < utf8.RuneSelf { // ASCII fast path: no rune decoding needed
			if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') ||
				runeExists(p.t.kwMajorSymbols, rune(c)) ||
				(start != -1 && runeExists(p.t.kwMinorSymbols, rune(c))) {
				if start == -1 {
					start = p.pos
				}
				p.next()
				continue
			}
			break
		}
		var r rune
		var size int
		for p.reader != nil && !utf8.FullRune(p.str[p.pos:]) { // the rune may be split between chunks
			p.loadChunk()
		}
		r, size = utf8.DecodeRune(p.str[p.pos:])
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
		} else if p.curr == '_' {
			if !hasNumber || (!p.t.allowNumberUnderscore || !isNumberByte(p.nextByte())) {
				break
			}
		} else if p.curr == '.' {
			nextByte := p.nextByte()
			if hasPoint {
				break
			} else if isNumberByte(nextByte) {
				if start == -1 { // floats can be started from a pointer
					start = p.pos
				}
			} else if !((nextByte == 'e' || nextByte == 'E') && hasNumber) && nextByte != 0 {
				break
			}
			floatTraitPos = p.pos
			end = p.pos
			hasPoint = true
		} else if p.curr == 'e' || p.curr == 'E' {
			nextByte := p.nextByte()
			if !hasNumber || !(isNumberByte(nextByte) || nextByte == '-' || nextByte == '+') || hasExp {
				break
			}
			floatTraitPos = p.pos
			hasExp = true
			hasPoint = true
		} else if hasExp && (p.curr == '-' || p.curr == '+') {
			if isNumberByte(p.nextByte()) {
				if start == -1 { // numbers can be started from a sign
					start = p.pos
				}
			} else {
				break
			}
		} else {
			break
		}
		p.next()
	}
	if start == -1 {
		return false
	}
	end = end + 1
	if p.pos != end { // roll back the bytes that are not a part of the number, e.g. "e+" in "1e+"
		p.pos = end
		p.curr = p.str[end]
	}
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
	var injectTokens [][]*tokenRef
	if len(quote.Injects) > 0 {
		injectTokens = make([][]*tokenRef, len(quote.Injects))
		for i, inject := range quote.Injects {
			injectTokens[i] = p.t.tokens[inject.StartKey]
		}
	}
	escapes := false
chars:
	for p.curr != 0 {
		if escapes {
			escapes = false
		} else if p.curr == quote.EscapeSymbol {
			escapes = true
		} else if p.match(quote.EndToken, true) {
			break
		} else if quote.Injects != nil {
			for i, inject := range quote.Injects {
				for _, token := range injectTokens[i] {
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
						// p.curr is the first byte after the injection (or 0 at the end of data): check it, don't skip it
						continue chars
					}
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
