package tokenizer

import (
	"strconv"
	"strings"
)

// Stream iterator via parsed tokens.
// If data reads from an infinite buffer, then the iterator will be read data from the reader chunk-by-chunk.
type Stream struct {
	t *Tokenizer
	// count of tokens in the stream
	len int
	// pointer to the node of a token double-linked list
	current *Token
	// pointer of valid token if current moved to out of bounds (out of end the list)
	prev *Token
	// pointer of valid token if current moved to out of bounds (out of begin the list)
	next *Token
	// pointer to head of the list
	head *Token

	// last whitespaces before the end of a source
	wsTail []byte
	// count of parsed bytes
	parsed int

	p           *parsing
	historySize int
}

func validateToken(t *Token) *Token {
	if t != nil {
		return t
	}
	return undefToken
}

// NewStream creates a new parsed stream of tokens.
func NewStream(p *parsing) *Stream {
	return &Stream{
		t:       p.t,
		head:    validateToken(p.head),
		current: validateToken(p.head),
		len:     p.n,
		wsTail:  p.tail,
		parsed:  p.parsed + p.pos,
	}
}

// NewInfStream creates new stream with active parser.
func NewInfStream(p *parsing) *Stream {
	return &Stream{
		t:       p.t,
		p:       p,
		len:     p.n,
		head:    validateToken(p.head),
		current: validateToken(p.head),
	}
}

// SetHistorySize sets the number of tokens that should remain after the current token
func (s *Stream) SetHistorySize(size int) *Stream {
	s.historySize = size
	return s
}

// Close releases all token objects to pool
func (s *Stream) Close() {
	for ptr := s.head; ptr != nil && ptr != undefToken; {
		p := ptr.next
		s.t.freeToken(ptr)
		ptr = p
	}
	s.next = nil
	s.prev = nil
	s.head = undefToken
	s.current = undefToken
	s.len = 0
}

func (s *Stream) String() string {
	items := make([]string, 0, s.len)
	ptr := s.head
	for ptr != nil {
		items = append(items, strconv.Itoa(ptr.id)+": "+ptr.String())
		ptr = ptr.next
	}

	return strings.Join(items, "\n")
}

// GetParsedLength returns currently count parsed bytes.
func (s *Stream) GetParsedLength() int {
	if s.p == nil {
		return s.parsed
	} else {
		return s.p.parsed + s.p.pos
	}
}

// GoNext moves the stream pointer to the next token.
// If there is no token, it initiates the parsing of the next chunk of data.
// If there is no data, the pointer will point to the TokenUndef token.
func (s *Stream) GoNext() *Stream {
	if s.current.next != nil {
		s.current = s.current.next
		if s.current.next == nil && s.p != nil { // lazy load and parse next data-chunk
			n := s.p.n
			s.p.parse()
			s.len += s.p.n - n
		}
		if s.historySize != 0 && s.current.id-s.head.id > s.historySize {
			t := s.head
			s.head = s.head.unlink()
			s.t.freeToken(t)
			s.len--
		}
	} else if s.current == undefToken {
		s.current = s.prev
		s.prev = nil
	} else {
		s.prev = s.current
		s.current = undefToken
	}
	return s
}

// GoPrev moves the pointer of stream to the next token.
// The number of possible calls is limited if you specified SetHistorySize.
// If the beginning of the stream or the end of the history is reached, the pointer will point to the TokenUndef token.
func (s *Stream) GoPrev() *Stream {
	if s.current.prev != nil {
		s.current = s.current.prev
	} else if s.current == undefToken {
		s.current = s.next
		s.prev = nil
	} else {
		s.next = s.current
		s.current = undefToken
	}
	return s
}

// GoTo moves the pointer of stream to specific token.
func (s *Stream) GoTo(id int) *Stream {
	if s.current == undefToken {
		if s.prev != nil && id <= s.prev.id { // we at the end of the stream
			s.GoPrev() // now current is available
			for s.current != nil && id != s.current.id {
				s.GoPrev()
			}
		} else if s.next != nil && id >= s.prev.id { // we at the beginning of the stream
			s.GoNext() // now current is available
			for s.current != nil && id != s.current.id {
				s.GoNext()
			}
		}
		return s
	}
	if id > s.current.id {
		for s.current != nil && id != s.current.id {
			s.GoNext()
		}
	} else if id < s.current.id {
		for s.current != nil && id != s.current.id {
			s.GoPrev()
		}
	}
	return s
}

// IsValid checks if stream is valid.
// This means that the pointer has not reached the end of the stream.
func (s *Stream) IsValid() bool {
	return s.current != nil && s.current != undefToken
}

// IsNextSequence checks if these are next tokens in exactly the same sequence as specified.
func (s *Stream) IsNextSequence(keys ...TokenKey) bool {
	var (
		result = true
		hSize  = 0
	)
	if s.historySize > 0 && s.historySize < len(keys) {
		hSize = s.historySize
		s.historySize = len(keys)
	}

	start := s.current
	for _, key := range keys {
		if !s.GoNext().CurrentToken().Is(key) {
			result = false
			break
		}
	}
	s.current = start

	if hSize != 0 {
		s.SetHistorySize(hSize)
	}
	return result
}

// IsAnyNextSequence checks that at least one token from each group is contained in a sequence of tokens
func (s *Stream) IsAnyNextSequence(keys ...[]TokenKey) bool {
	var (
		result = true
		hSize  = 0
	)
	if s.historySize > 0 && s.historySize < len(keys) {
		hSize = s.historySize
		s.historySize = len(keys)
	}

	start := s.current
	for _, key := range keys {
		found := false
		for _, k := range key {
			if s.GoNext().CurrentToken().Is(k) {
				found = true
				break
			}
		}
		if !found {
			result = false
			break
		}
	}
	s.current = start

	if hSize != 0 {
		s.SetHistorySize(hSize)
	}
	return result
}

// HeadToken returns the pointer to head-token.
// Parser may change Head token if history size is enabled.
func (s *Stream) HeadToken() *Token {
	return s.head
}

// CurrentToken always returns the token.
// If the pointer is not valid (see IsValid), CurrentToken will be return TokenUndef token.
// Do not save result (Token) into variables — current token may be changed at any time.
func (s *Stream) CurrentToken() *Token {
	return s.current
}

// PrevToken returns previous token from the stream.
// If the previous token doesn't exist, the method returns TypeUndef token.
// Do not save a result (Token) into variables — the previous token may be changed at any time.
func (s *Stream) PrevToken() *Token {
	if s.current.prev != nil {
		return s.current.prev
	}
	return undefToken
}

// NextToken returns next token from the stream.
// If a next token doesn't exist, the method returns TypeUndef token.
// Do not save a result (Token) into variables — the next token may be changed at any time.
func (s *Stream) NextToken() *Token {
	if s.current.next != nil {
		return s.current.next
	}
	return undefToken
}

// GoNextIfNextIs moves the stream pointer to the next token if the next token has specific token keys.
// If a key matched pointer is updated and the method returns true. Otherwise, returns false.
func (s *Stream) GoNextIfNextIs(key TokenKey, otherKeys ...TokenKey) bool {
	if s.NextToken().Is(key, otherKeys...) {
		s.GoNext()
		return true
	}
	return false
}

// GetSnippet returns slice of tokens.
// Slice generated from a current token position and include a number of tokens before and after the current token.
func (s *Stream) GetSnippet(before, after int) []Token {
	if s.current == nil {
		return nil
	}
	snippet := make([]Token, before+after+1)
	start := 0
	end := before + after
	snippet[before] = Token{
		id:     s.current.id,
		key:    s.current.key,
		value:  s.current.value,
		line:   s.current.line,
		offset: s.current.offset,
		indent: s.current.indent,
		string: s.current.string,
	}
	if s.current.prev != nil && before > 0 {
		ptr := s.current.prev
		for i := 1; i <= before; i++ {
			snippet[before-i] = Token{
				id:     ptr.id,
				key:    ptr.key,
				value:  ptr.value,
				line:   ptr.line,
				offset: ptr.offset,
				indent: ptr.indent,
				string: ptr.string,
			}
			ptr = ptr.prev
			if ptr == nil {
				start = before - i
				break
			}
		}
	}
	if s.current.next != nil && after > 0 {
		ptr := s.current.next
		for i := 1; i <= after; i++ {
			snippet[before+i] = Token{ // before - is offset
				id:     ptr.id,
				key:    ptr.key,
				value:  ptr.value,
				line:   ptr.line,
				offset: ptr.offset,
				indent: ptr.indent,
				string: ptr.string,
			}
			ptr = ptr.next
			if ptr == nil {
				end = -i
				break
			}
		}
	}
	if start == 0 && end == before+after {
		return snippet
	}
	return snippet[start:end]
}

// GetSnippetAsString returns tokens before and after a current token as string.
// `maxStringLength` specifies max length of each token string.
// Zero — unlimited token string length.
// If a string is greater than maxLength method removes some runes in the middle of the string.
func (s *Stream) GetSnippetAsString(before, after, maxStringLength int) string {
	segments := s.GetSnippet(before, after)
	str := make([]string, len(segments))
	for i, token := range segments {
		v := token.ValueString()
		if maxStringLength > 4 && len(v) > maxStringLength {
			str[i] = v[:maxStringLength/2] + "..." + v[maxStringLength/2+1:]
		} else {
			str[i] = v
		}
	}

	return strings.Join(str, "")
}
