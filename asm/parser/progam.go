package parser

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"go.creack.net/corewar/op"
)

type Program struct {
	*Parser

	buf              []byte
	idx              int
	labels           map[string]int
	hasLabelIndex    bool
	hasMissingLabels bool

	extendModeEnabled bool
	strict            bool
}

func NewProgram(p *Parser, strict bool) *Program {
	return &Program{
		Parser: p,

		buf:               make([]byte, op.MemSize),
		idx:               0,
		labels:            nil, // Keeping as nil to indicate that we don't have any labels yet.
		hasLabelIndex:     false,
		hasMissingLabels:  false,
		extendModeEnabled: false,
		strict:            strict,
	}
}

func (p Program) Size() int {
	return p.idx
}

func (p *Program) encode() error {
	// If we have labels, it means we already encoded once and have the labels index.
	// Error out if we encounter a label that we don't know
	p.hasLabelIndex = p.labels != nil
	if !p.hasLabelIndex {
		p.labels = map[string]int{}
	}
	p.idx = 0
	for _, n := range p.Parser.Nodes {
		if _, err := n.Encode(p); err != nil {
			return fmt.Errorf("failed to encode instruction %s: %w", n, err)
		}
	}

	return nil
}

func (p *Program) Encode() ([]byte, error) {
	if err := p.encode(); err != nil {
		return nil, fmt.Errorf("failed to first encode program: %w", err)
	}

	// If we don't have any missing labels, we don't need to re-encode.
	if !p.hasMissingLabels {
		return p.buf[:p.idx], nil
	}

	// If we have missing labels, we need to re-encode the program.
	if err := p.encode(); err != nil {
		return nil, fmt.Errorf("failed to re-encode program: %w", err)
	}

	return p.buf[:p.idx], nil
}

func (p *Program) Decode(data []byte, strict bool) (*Parser, error) {
	if len(data) > op.MemSize {
		return nil, fmt.Errorf("program size %d exceeds memory size %d", len(data), op.MemSize)
	}
	p.buf = make([]byte, len(data))
	copy(p.buf, data)

	p.Parser = &Parser{}
	if err := p.DecodeHeader(strict); err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	for p.idx < len(p.buf) {
		ins, idx, err := DecodeNextInstruction(p.buf[p.idx:])
		if err != nil {
			if errors.Is(err, ErrInvalidOpcode) {
				// TODO: Handle this. Likely .code directive.
				// Look for the offset with the most valid instructions,
				// as the raw code could contain part of the instruction.
				p.idx++
				continue
			}
			return nil, fmt.Errorf("failed to decode instruction: %w", err)
		}
		p.idx += idx
		p.Parser.Nodes = append(p.Parser.Nodes, ins)
	}
	return p.Parser, nil
}

func (p *Program) DecodeHeader(strict bool) error {
	h := op.Header{}
	headerSize, nameLength, commentLength := op.HeaderStructSize()

	if len(p.buf) < headerSize {
		return fmt.Errorf("invalid header size")
	}

	p.idx = 0
	h.Magic = op.Endian.Uint32(p.buf[p.idx : p.idx+4])
	p.idx += 4
	if h.Magic != op.CorewarExecMagic {
		if strict {
			return fmt.Errorf("invalid magic number: %x, expect %x", h.Magic, op.CorewarExecMagic)
		}
		log.Printf("Warning: invalid magic number: %x, expect %x", h.Magic, op.CorewarExecMagic)
	}
	copy(h.ProgName[:], p.buf[p.idx:p.idx+nameLength])
	p.idx += nameLength

	h.ProgSize = op.Endian.Uint32(p.buf[p.idx : p.idx+4])
	p.idx += 4

	copy(h.Comment[:], p.buf[p.idx:p.idx+commentLength])
	p.idx += commentLength

	if p.idx >= len(p.buf) {
		return fmt.Errorf("no code after header")
	}

	if p.idx+int(h.ProgSize) != len(p.buf) {
		if strict {
			return fmt.Errorf("program size from header doesn't match actual code size, header: %d, actual: %d", h.ProgSize, len(p.buf)-p.idx)
		}
		log.Printf("Warning: program size from header doesn't match actual code size, header: %d, actual: %d\n", h.ProgSize, len(p.buf)-p.idx)
	}

	p.Parser.Nodes = append(p.Parser.Nodes, &Directive{
		Name:  strings.TrimPrefix(op.NameCmdString, string(op.DirectiveChar)),
		Value: cStrToString(h.ProgName[:]),
	})

	if comment := cStrToString(h.Comment[:]); comment != "" {
		p.Parser.Nodes = append(p.Parser.Nodes, &Directive{
			Name:  strings.TrimPrefix(op.CommentCmdString, string(op.DirectiveChar)),
			Value: comment,
		})
	}

	return nil
}

func cStrToString(cstr []byte) string {
	for i, b := range cstr {
		if b == 0 {
			return string(cstr[:i])
		}
	}
	return string(cstr)
}
