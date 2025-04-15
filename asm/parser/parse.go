package parser

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"go.creack.net/corewar/op"
)

// Parameter represents a parameter in an instruction.
// Stored as raw string.
type Parameter struct {
	Typ   op.ParamType
	Value string
}

func (p Parameter) String() string {
	switch p.Typ {
	case op.TReg:
		return string(op.RegisterChar) + p.Value
	case op.TInd:
		return p.Value
	case op.TDir:
		return string(op.DirectChar) + p.Value
	case op.TLab:
		return string(op.LabelChar) + p.Value
	default:
		return fmt.Sprintf("unknown param type %d", p.Typ)
	}
}

// Encode the parameter in the given buffer based
// on the paramter type and address mode (from the instruction opcode).
// If the given buffer is nil, it will not write anything but still return
// how many bytes would have been written.
func (p Parameter) Encode(buf []byte, addrMode op.AddressMode) (int, error) {
	// If the value is a label reference, we don't have the value
	// yet, just return the size of the label.
	if strings.HasPrefix(p.Value, string(op.LabelChar)) {
		return 2, nil
	}

	// Parse the value as a number.
	n, err := strconv.Atoi(p.Value)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", p.Value, err)
	}
	//log.Fatalf("%q: %d\n", p.Value, n)
	// Simplest case, register.
	if p.Typ == op.TReg {
		if n < 1 || n > op.RegisterCount {
			return 0, fmt.Errorf("invalid register number %d", n)
		}
		if buf != nil {
			buf[0] = byte(n)
		}
		return 1, nil
	}

	// TODO: Handle TInd?.
	switch addrMode {
	case op.AddrModeIndex:
		if buf != nil {
			op.Endian.PutUint16(buf, uint16(n))
		}
		return 2, nil
	case op.AddrModeValue:
		if buf != nil {
			op.Endian.PutUint32(buf, uint32(n))
		}
		return 4, nil
	default:
		return 0, fmt.Errorf("unexpected address mode %d for parameter %q", addrMode, p)
	}
}

type Instruction struct {
	Labels []string    // Optional labels.
	OpCode op.OpCode   // OpCode reference.
	Params []Parameter // Parameters.
}

// Generate the encoding byte for the instruction based on
// the parameters types.
func (ins Instruction) ParamsEncoding() byte {
	// Each instruction type is represented by 2 bits.
	// To represent the type of the instruction,
	// we set the first (leftmost) 2 bits to the first parameter type,
	// the second 2 bits to the second parameter type, and so on.
	out := byte(0)
	for i, p := range ins.Params {
		// For the first iteration, we have a type 'xx',
		// which we can represent '000000xx',
		// to place it in the first 2 bits, we need to shift it
		// to the left by (3 - i) * 2 bits, i.e (3 - 0) * 2, i.e. 6.
		// We then have 'xx000000'.
		// The second iteration, will be 4, then 2, then 0.
		out |= (byte(p.Typ) << ((3 - i) * 2))
	}
	return out
}

func (ins Instruction) String() string {
	out := ""
	for _, label := range ins.Labels {
		out += label + string(op.LabelChar) + "\n"
	}
	out += "\t" + ins.OpCode.Name
	paramStrs := make([]string, 0, len(ins.Params))
	for _, param := range ins.Params {
		paramStrs = append(paramStrs, param.String())
	}
	out += " " + strings.Join(paramStrs, string(op.SeparatorChar)+" ")
	return out
}

func (ins Instruction) ValidateParameters() error {
	if len(ins.Params) != len(ins.OpCode.ParamTypes) {
		return fmt.Errorf("expected %d parameters, got %d", len(ins.OpCode.ParamTypes), len(ins.Params))
	}
	for i, param := range ins.Params {
		// Check that `param.Typ` bytes is within the ins.OpCode.ParamTypes[i] mask.
		if param.Typ&ins.OpCode.ParamTypes[i] == 0 {
			return fmt.Errorf("invalid %d parameter type %q for %q, expect %q", i+1, param.Typ, ins.OpCode.Name, ins.OpCode.ParamTypes[i])
		}
	}

	for _, param := range ins.Params {
		if param.Value == "" {
			return fmt.Errorf("parameter value cannot be empty")
		}
	}

	return nil
}

// Parser structure
type Parser struct {
	lexer     *lexer
	currToken item
	peekToken item

	Directives     map[string]string
	Instructions   []*Instruction
	curInstruction *Instruction
}

// NewParser creates a new parser
func NewParser(name, input string) *Parser {
	p := &Parser{
		lexer:      NewLexer(name, input),
		Directives: map[string]string{},
	}
	// Preload the next token.
	p.nextToken()
	return p
}

func (p *Parser) parseDirective() error {
	directiveName := strings.TrimPrefix(p.currToken.val, string(op.DirectiveChar))
	p.nextToken()

	// Case where we don't have a directive value (e.g. .extend)
	if p.currToken.typ.isEOL() {
		if _, ok := p.Directives[directiveName]; ok {
			return fmt.Errorf("duplicate directive %q", directiveName)
		}
		p.Directives[directiveName] = ""
		return nil
	}

	if p.currToken.typ != itemRawString {
		return fmt.Errorf("expected string, got %s for %q", p.currToken, directiveName)
	}
	directiveValue := strings.Trim(p.currToken.val, "\"")
	if _, ok := p.Directives[directiveName]; ok {
		return fmt.Errorf("duplicate directive %q", directiveName)
	}
	p.Directives[directiveName] = directiveValue
	return nil
}

func (p *Parser) parseIdentifier() error {
	if p.curInstruction == nil {
		ins := Instruction{}
		p.Instructions = append(p.Instructions, &ins)
		p.curInstruction = &ins
	}

	// Optional label in front or alone on the line.
	if p.peekToken.typ == itemColumn {
		for _, elem := range p.Instructions {
			for _, label := range elem.Labels {
				if label == p.currToken.val {
					// TODO: Embed the pos to report where was the other label.
					return fmt.Errorf("duplicate label %q", p.currToken.val)
				}
			}
		}
		p.curInstruction.Labels = append(p.curInstruction.Labels, p.currToken.val)
		p.nextToken()
		log.Printf("parser -1: %v -- %v\n", p.currToken, p.peekToken)
		return nil
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
		log.Printf("parser1: %v -- %v\n", p.currToken, p.peekToken)

		// If we have an identifier, it can be a register if it starts with 'r'
		// or an indirect if it is a number.
		if p.currToken.typ == itemIdentifier {
			if strings.HasPrefix(p.currToken.val, string(op.RegisterChar)) {
				param.Typ = op.TReg
				param.Value = p.currToken.val[1:]
			} else {
				// TODO: Add stronger validation, make sure it is a number.
				param.Typ = op.TInd
				param.Value = p.currToken.val
			}
			continue
		}
		// Indirect label.
		if p.currToken.typ == itemColumn {
			param.Typ = op.TInd
			p.nextToken()
			log.Printf("parser2: %v -- %v\n", p.currToken, p.peekToken)
			if p.currToken.typ != itemIdentifier {
				return fmt.Errorf("expected identifier for indirect label, got %s", p.currToken)
			}
			param.Value = p.currToken.val
			continue
		}

		// Direct value.
		if p.currToken.typ == itemPercent {
			param.Typ = op.TDir
			p.nextToken()
			log.Printf("parser3: %v -- %v\n", p.currToken, p.peekToken)
			if p.currToken.typ != itemIdentifier && p.currToken.typ != itemColumn {
				return fmt.Errorf("expected identifier or column for direct value, got %s", p.currToken)
			}
			if p.currToken.typ == itemColumn {
				p.nextToken()
				log.Printf("parser4: %v -- %v\n", p.currToken, p.peekToken)
				if p.currToken.typ != itemIdentifier {
					return fmt.Errorf("expected identifier for direct label, got %s", p.currToken)
				}
				param.Value = string(op.LabelChar) + p.currToken.val
			} else if p.currToken.typ == itemIdentifier {
				param.Value = p.currToken.val
			} else {
				return fmt.Errorf("expected identifier for direct value, got %s", p.currToken)
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
			if param != (Parameter{}) {
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
		default:
			return fmt.Errorf("unexpected item %s", item)
		}
		if err != nil {
			return fmt.Errorf("error parsing item: %s", err)
		}
	}

	return nil
}
