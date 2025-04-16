package parser

import (
	"fmt"
	"strings"

	"go.creack.net/corewar/op"
)

type Node interface {
	Encode(*Program) error
	PrettyPrint([]Node) string
}

// Parser structure
type Parser struct {
	lexer     *lexer
	currToken item
	peekToken item

	Nodes          []Node
	curInstruction *Instruction
}

// NewParser creates a new parser
func NewParser(name, input string) *Parser {
	p := &Parser{
		lexer: NewLexer(name, input),
	}
	// Preload the next token.
	p.nextToken()

	return p
}

// GetDirective returns the last value of the given directive key.
func (p *Parser) GetDirective(name string) string {
	var out string
	for _, elem := range p.Nodes {
		if d, ok := elem.(*Directive); ok && d.Name == name {
			out = d.Value
		}
	}
	return out
}

func (p *Parser) parseDirective() error {
	p.curInstruction = nil // If we reach a directive, we are not in an instruction anymore.

	directiveName := strings.TrimPrefix(p.currToken.val, string(op.DirectiveChar))
	p.nextToken()
	if p.currToken.typ == itemError {
		return fmt.Errorf("unexpected token %s", p.currToken)
	}

	d := &Directive{Name: directiveName}

	// If we have a raw string, use it as value, if we have EOL, the value is empty.
	if p.currToken.typ.isEOL() || p.currToken.typ == itemRawString {
		d.Value = strings.Trim(p.currToken.val, "\"")
		p.Nodes = append(p.Nodes, d)
		return nil
	}

	// The directive value must be a string or a number, as we accept hexadecimal without prefix, it can be an identifier.
	if p.currToken.typ != itemNumber && p.currToken.typ != itemIdentifier {
		return fmt.Errorf("expected number or identifier, got %s for %q", p.currToken, directiveName)
	}

	var values []string
	for !p.currToken.typ.isEOL() {
		if p.currToken.typ == itemError {
			return fmt.Errorf("unexpected token %s", p.currToken)
		}
		values = append(values, p.currToken.val)
		p.nextToken()
	}

	d.Value = strings.Join(values, " ")
	p.Nodes = append(p.Nodes, d)
	return nil
}

func (p *Parser) parseLabel() error {
	for _, elem := range p.Nodes {
		if l, ok := elem.(*Label); ok && l.Name == p.currToken.val {
			// TODO: Embed the pos to report where was the other label.
			return fmt.Errorf("duplicate label %q", p.currToken.val)
		}
	}
	p.Nodes = append(p.Nodes, &Label{Name: p.currToken.val})
	return nil
}

func (p *Parser) parseIdentifier() error {
	if p.curInstruction == nil {
		ins := Instruction{}
		p.Nodes = append(p.Nodes, &ins)
		p.curInstruction = &ins
	}

	// If we don't have the instruction name yet, we need it.
	if p.curInstruction.OpCode.Name == "" {
		if p.currToken.typ != itemIdentifier {
			return fmt.Errorf("expected instruction name, got %s", p.currToken)
		}
		for _, ins := range op.OpCodeTable {
			if ins.Name == p.currToken.val {
				p.curInstruction.OpCode = ins
				break
			}
		}
		if p.curInstruction.OpCode.Name == "" {
			return fmt.Errorf("unknown instruction %q", p.currToken.val)
		}
	}
	return p.parseInstructionParameters()
}

func (p *Parser) parseInstructionParameters() error {
	var param Parameter
	for {
		p.nextToken()

		if p.currToken.typ == itemError {
			return fmt.Errorf("unexpected token %s", p.currToken)
		}

		// If we have an identifier, it can be a register if it starts with 'r'
		// otherwise, it is am error.
		if p.currToken.typ == itemIdentifier {
			if strings.HasPrefix(p.currToken.val, string(op.RegisterChar)) {
				param.Typ = op.TReg
				param.Value = p.currToken.val[1:]
			} else {
				return fmt.Errorf("unexpected identifier %q in %q", p.currToken, p.curInstruction)
			}
			continue
		}
		// If we have a number, it may be an indirection or an arithmetic operation.
		if p.currToken.typ == itemNumber {
			if param.Typ == 0 { // If we don't have a type yet, it means we are first up, dealing with an indirection.
				param.Typ = op.TInd
				param.Value = p.currToken.val
			} else { // If we have a type, it means we are dealing with an arithmetic operation.
				param.Modifiers = append(param.Modifiers, modifier{raw: p.currToken.val})
			}
			continue
		}

		// Indirect label.
		if p.currToken.typ == itemLabelRef {
			if param.Typ == 0 {
				param.Typ = op.TInd
				param.Value = string(op.LabelChar) + p.currToken.val
			} else {
				param.Modifiers = append(param.Modifiers, modifier{raw: string(op.LabelChar) + p.currToken.val})
			}
			continue
		}

		// Direct value.
		if p.currToken.typ == itemPercent {
			if param.Typ != 0 {
				return fmt.Errorf("unexpected %s in %q", p.currToken, p.curInstruction)
			}
			param.Typ = op.TDir
			p.nextToken()

			// We expect either a column (label reference prefix) or a number.
			if p.currToken.typ == itemLabelRef {
				param.Value = string(op.LabelChar) + p.currToken.val
			} else if p.currToken.typ == itemNumber {
				param.Value = p.currToken.val
			} else {
				return fmt.Errorf("expected number or label reference for direct value, got %s", p.currToken)
			}
			continue
		}

		if p.currToken.typ == itemComa {
			p.curInstruction.Params = append(p.curInstruction.Params, param)
			param = Parameter{}
			if p.peekToken.typ.isEOL() {
				return fmt.Errorf("unexpected comma at the end of instruction %s", p.curInstruction)
			}
			continue
		}

		// If we reach EOF or newline, we are at the end of the instruction.
		// Append the last parameter if it exists and validate.
		if p.currToken.typ.isEOL() {
			if param.Typ != 0 {
				p.curInstruction.Params = append(p.curInstruction.Params, param)
			}

			if err := p.curInstruction.ValidateParameters(); err != nil {
				return fmt.Errorf("invalid instruction %s: %w", p.curInstruction, err)
			}

			// Mark curInstruction as nil, the next parseIdentifier call will set it
			// to the new one.
			p.curInstruction = nil
			break
		}

		return fmt.Errorf("unexpected token %s", p.currToken)
	}
	return nil
}

// nextToken advances to the next token
func (p *Parser) nextToken() {
	p.currToken = p.peekToken
	p.peekToken = p.lexer.nextItem()
}

func (p *Parser) Parse() error {
	for {
		p.nextToken()
		item := p.currToken
		if item.typ == itemEOF {
			break
		}
		if item.typ == itemError {
			return fmt.Errorf("[%d:%d]: %s", item.line, item.pos, item.val)
		}

		var err error
		switch item.typ {
		case itemNewline:
			continue
		case itemDirective:
			err = p.parseDirective()
		case itemComment:
			continue
		case itemIdentifier:
			err = p.parseIdentifier()
		case itemLabel:
			err = p.parseLabel()
		default:
			return fmt.Errorf("unexpected item %s", item)
		}
		if err != nil {
			return fmt.Errorf("error parsing item: %s", err)
		}
	}

	return nil
}
