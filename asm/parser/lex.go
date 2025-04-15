package parser

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"go.creack.net/corewar/op"
)

type stateFn func(*lexer) stateFn

const eof = -1

type itemType int

const (
	itemError itemType = iota // Error occurred; value is text of (a single) error.
	itemNewline
	itemIdentifier
	itemComment
	itemRawString // Raw string, including quotes.
	itemColumn
	itemComa
	itemPercent
	itemEOF // End of the input.

	// Keywords appear after all the rest.
	itemKeyword // Used only to delimit the keywords.
	itemDirective
)

func (it itemType) String() string {
	switch it {
	case itemError:
		return "<error>"
	case itemNewline:
		return "<newline>"
	case itemIdentifier:
		return "<identifier>"
	case itemComment:
		return "<comment>"
	case itemRawString:
		return "<raw string>"
	case itemColumn:
		return "<column>"
	case itemComa:
		return "<coma>"
	case itemPercent:
		return "<percent>"
	case itemEOF:
		return "<eof>"
	case itemKeyword:
		return "<keyword>"
	case itemDirective:
		return "<directive>"
	default:
		return fmt.Sprintf("<unknown token %d>", it)
	}
}

func (it itemType) isEOL() bool {
	// NOTE: We only support whole-line comments, i.e., no /* */,
	//       so any comment means end of line.
	return it == itemNewline || it == itemEOF || it == itemComment
}

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
	case i.typ == itemNewline:
		return "'\\n'"
	case i.typ > itemKeyword:
		return fmt.Sprintf("<%s>", i.val)
	case len(i.val) > 10:
		return fmt.Sprintf("%s %.10q...", i.typ, i.val)
	}
	return fmt.Sprintf("%s %q", i.typ, i.val)
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
	l.acceptRun(" \t") // Consume leading whitespace.
	if l.atEOF {
		return l.emit(itemEOF)
	}
	l.ignore() // ignore leading whitespace.
	switch r := l.peek(); r {
	case '\n':
		l.acceptRun(" \t\n")
		l.ignore()
		if l.atEOF {
			return l.emit(itemEOF)
		}
		return l.emit(itemNewline)
	case op.DirectiveChar:
		return lexDirective
	case '"':
		return lexString
	case op.LabelChar:
		l.pos++
		return l.emit(itemColumn)
	case op.SeparatorChar:
		l.pos++
		return l.emit(itemComa)
	case op.DirectChar:
		l.pos++
		return l.emit(itemPercent)
	case '-', '+':
		l.pos++
		return lexIdentifier
	default:
		if strings.ContainsRune(op.CommentChars, r) {
			return lexComment
		}

		// NOTE: All instruction chars are within the label set.
		if strings.ContainsRune(op.LabelChars, r) {
			return lexIdentifier
		}
		return l.errorf("unexpected character %c", r)
	}
}

func lexIdentifier(l *lexer) stateFn {
	l.acceptRun("-+")
	l.acceptRun(op.LabelChars) // NOTE: All instruction chars are within the label set.
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
	i := l.thisItem(itemComment)
	i.val = strings.TrimSpace(i.val)
	return l.emitItem(i)
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
	nextSpace := strings.IndexAny(l.input[l.start:], " \t\n")
	if nextSpace == -1 {
		return l.errorf("missing space after directive")
	}
	l.pos += Pos(nextSpace)
	i := l.thisItem(itemDirective)
	if i.val == string(op.DirectiveChar) {
		return l.errorf("missing directive name")
	}
	return l.emitItem(i)
}

// nextItem returns the next item from the input.
// Called by the parser, not in the lexing goroutine.
func (l *lexer) nextItem() item {
	l.item = item{itemEOF, l.pos, "EOF", l.startLine}
	state := lexText
	for {
		state = state(l)
		if state == nil {
			//fmt.Printf("lexer: %s\n", l.item)
			//time.Sleep(100e6)
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
