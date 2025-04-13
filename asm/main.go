package main

import (
	"fmt"
	"log"
	"strings"
	"unicode/utf8"
)

// op_t    op_tab[]=
// {
//   {"live",1,{T_DIR},1,10,"alive"},
//   {"ld",2,{T_DIR|T_IND,T_REG},2,5,"load"},
//   {"st",2,{T_REG,T_IND|T_REG},3,5,"store"},
//   {"add",3,{T_REG,T_REG,T_REG},4,10,"addition"},
//   {"sub",3,{T_REG,T_REG,T_REG},5,10,"soustraction"},
//   {"and",3,{T_REG|T_DIR|T_IND,T_REG|T_IND|T_DIR,T_REG},6,6,"et (and  r1,r2,r3   r1&r2 -> r3"},
//   {"or",3,{T_REG|T_IND|T_DIR,T_REG|T_IND|T_DIR,T_REG},7,6,"ou  (or   r1,r2,r3   r1|r2 -> r3"},
//   {"xor",3,{T_REG|T_IND|T_DIR,T_REG|T_IND|T_DIR,T_REG},8,6,"ou (xor  r1,r2,r3   r1^r2 -> r3"},
//   {"zjmp",1,{T_DIR},9,20,"jump if zero"},
//   {"ldi",3,{T_REG|T_DIR|T_IND,T_DIR|T_REG,T_REG},10,25,"load index"},
//   {"sti",3,{T_REG,T_REG|T_DIR|T_IND,T_DIR|T_REG},11,25,"store index"},
//   {"fork",1,{T_DIR},12,800,"fork"},
//   {"lld",2,{T_DIR|T_IND,T_REG},13,10,"long load"},
//   {"lldi",3,{T_REG|T_DIR|T_IND,T_DIR|T_REG,T_REG},14,50,"long load index"},
//   {"lfork",1,{T_DIR},15,1000,"long fork"},
//   {"aff",1,{T_REG},16,2,"aff"},
//   {0,0,0,0,0}
// };

const sample = `
.name "zork"
.comment "just a basic living prog"
l2: sti r1,%:live,%1
and r1,%0,r1
live: live %1
zjmp %:live
# Executable compile (after header):
#
# 0x0b,0x68,0x01,0x00,0x0f,0x00,0x01
# 0x06,0x64,0x01,0x00,0x00,0x00,0x00,0x01
# 0x01,0x00,0x00,0x00,0x01
# 0x09,0xff,0xfb
`

const (
	TokenRoot = iota
	TokenHeaderName
	TokenString
)

const eof = -1

type itemType int

const (
	itemError itemType = iota // Error occurred; value is text of (a single) error.
	itemIdentifier
	itemComment
	itemRawString // Raw string, including quotes.
	itemSpace     // Run of spaces.
	itemColumn
	itemComa
	itemPercent
	itemEOF // End of the input.

	// Keywords appear after all the rest.
	itemKeyword // Used only to delimit the keywords.
	itemDirective
)

type item struct {
	typ  itemType // The type of this item.
	pos  Pos      // The start position, in bytes, of this item in the input string.
	val  string   // The value of this item.
	line int      // The line number at the start of this item.
}

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	case i.typ > itemKeyword:
		return fmt.Sprintf("<%s>", i.val)
	case len(i.val) > 10:
		return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

type Pos int

// lexer hols the state of the scanner.
// lexer holds the state of the scanner.
type lexer struct {
	name      string // The name of the input; used only for error reports.
	input     string // The string being scanned.
	pos       Pos    // Current position in the input.
	start     Pos    // Start position of this item.
	atEOF     bool   // We have hit the end of input and returned eof.
	line      int    // 1+number of newlines seen.
	startLine int    // Start line of this item.
	item      item   // Item to return to parser.
	options   lexOptions
}

// lexOptions control behavior of the lexer. All default to false.
type lexOptions struct {
	emitComment bool // emit itemComment tokens.
	breakOK     bool // break keyword allowed
	continueOK  bool // continue keyword allowed
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...any) stateFn {
	l.item = item{itemError, l.start, fmt.Sprintf(format, args...), l.startLine}
	l.start = 0
	l.pos = 0
	l.input = l.input[:0]
	return nil
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.atEOF = true
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += Pos(w)
	if r == '\n' {
		l.line++
	}
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune.
func (l *lexer) backup() {
	if !l.atEOF && l.pos > 0 {
		r, w := utf8.DecodeLastRuneInString(l.input[:l.pos])
		l.pos -= Pos(w)
		// Correct newline count.
		if r == '\n' {
			l.line--
		}
	}
}

// thisItem returns the item at the current input point with the specified type
// and advances the input.
func (l *lexer) thisItem(t itemType) item {
	i := item{t, l.start, l.input[l.start:l.pos], l.startLine}
	l.start = l.pos
	l.startLine = l.line
	return i
}

// emit passes the trailing text as an item back to the parser.
func (l *lexer) emit(t itemType) stateFn {
	return l.emitItem(l.thisItem(t))
}

// emitItem passes the specified item to the parser.
func (l *lexer) emitItem(i item) stateFn {
	l.item = i
	return nil
}

// ignore skips over the pending input before this point.
// It tracks newlines in the ignored text, so use it only
// for text that is skipped without calling l.next.
func (l *lexer) ignore() {
	l.line += strings.Count(l.input[l.start:l.pos], "\n")
	l.start = l.pos
	l.startLine = l.line
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

// lexText scans until an opening action delimiter, "{{".
func lexText(l *lexer) stateFn {
	// Consume any leading whitespace.
	l.acceptRun(" \t\n")
	if l.atEOF {
		return l.emit(itemEOF)
	}
	l.ignore() // Discard the leading whitespaces.

	switch r := l.peek(); r {
	case '.':
		return lexDirective
	case '#':
		return lexComment
	case '"':
		return lexString
	case ':':
		l.pos++
		return l.emit(itemColumn)
	case ',':
		l.pos++
		return l.emit(itemComa)
	case '%':
		l.pos++
		return l.emit(itemPercent)
	default:
		// NOTE: All instruction chars are within the label set.
		if strings.ContainsRune(LabelChars, r) {
			return lexIdentifier
		}
		return l.errorf("unexpected character %q", r)
	}
}

func lexIdentifier(l *lexer) stateFn {
	l.acceptRun(LabelChars) // NOTE: All instruction chars are within the label set.
	if l.atEOF {
		return l.emit(itemEOF)
	}
	return l.emit(itemIdentifier)
}

func lexComment(l *lexer) stateFn {
	for {
		r := l.next()
		if r == eof || r == '\n' {
			break
		}
	}
	return l.emit(itemComment)
}

func lexString(l *lexer) stateFn {
	l.pos++
	for {
		r := l.next()
		if r == eof || r == '\n' {
			return l.errorf("missing closing quote")
		}
		if r == '"' {
			break
		}
		if r == '\\' {
			l.next()
		}
	}
	return l.emit(itemRawString)
}

func lexDirective(l *lexer) stateFn {
	fmt.Printf("lexDirective: %q (%d:%d)\n", l.input[l.start:l.pos+5], l.start, l.pos)
	nextSpace := strings.IndexAny(l.input[l.start:], " \t\n")
	if nextSpace == -1 {
		return l.errorf("missing space after directive")
	}
	l.pos += Pos(nextSpace)
	return l.emit(itemDirective)
}

type stateFn func(*lexer) stateFn

// nextItem returns the next item from the input.
// Called by the parser, not in the lexing goroutine.
func (l *lexer) nextItem() item {
	l.item = item{itemEOF, l.pos, "EOF", l.startLine}
	state := lexText
	for {
		state = state(l)
		if state == nil {
			return l.item
		}
	}
}

// lex creates a new scanner for the input string.
func NewLexer(name, input string) *lexer {
	return &lexer{
		name:      name,
		input:     input,
		line:      1,
		startLine: 1,
	}
}

// Parser structure
type Parser struct {
	lexer      *lexer
	directives map[string]string
}

// NewParser creates a new parser
func NewParser(name, input string) *Parser {
	return &Parser{
		lexer:      NewLexer(name, input),
		directives: map[string]string{},
	}
}

func (p *Parser) Parse() error {
	for {
		item := p.lexer.nextItem()
		if item.typ == itemEOF {
			break
		}
		if item.typ == itemError {
			return fmt.Errorf("error: %s", item.val)
		}
		fmt.Printf(">item: %s\n", item)
	}

	return nil
}

func main() {
	if err := NewParser("example", sample).Parse(); err != nil {
		log.Fatalf("fail: %s.", err)
	}
}
