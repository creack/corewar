package parser

import (
	"fmt"

	"go.creack.net/corewar/op"
)

type Program struct {
	p *Parser

	buf              []byte
	idx              int
	labels           map[string]int
	hasLabelIndex    bool
	hasMissingLabels bool

	extendModeEnabled bool
}

func NewProgram(p *Parser) *Program {
	return &Program{
		p: p,

		buf:               make([]byte, op.MemSize),
		idx:               0,
		labels:            nil, // Keeping as nil to indicate that we don't have any labels yet.
		hasLabelIndex:     false,
		hasMissingLabels:  false,
		extendModeEnabled: false,
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
	for _, n := range p.p.Nodes {
		if err := n.Encode(p); err != nil {
			return fmt.Errorf("failed to encode instruction %s: %w", n, err)
		}
	}

	return nil
}

func (p *Program) Encode() ([]byte, error) {
	if err := p.encode(); err != nil {
		return nil, fmt.Errorf("failed to first encode program: %w", err)
	}

	// If we don't have any missing labels, we don't need to re-encode,
	// return nil label map to indicate that.
	if !p.hasMissingLabels {
		return p.buf[:p.idx], nil
	}

	// If we have missing labels, we need to re-encode the program.
	if err := p.encode(); err != nil {
		return nil, fmt.Errorf("failed to re-encode program: %w", err)
	}

	return p.buf[:p.idx], nil
}
