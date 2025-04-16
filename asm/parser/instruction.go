package parser

import (
	"fmt"
	"strconv"
	"strings"

	"go.creack.net/corewar/op"
)

type Instruction struct {
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
		out |= p.Typ.Encoding() << ((3 - i) * 2)
	}
	return out
}

func (ins Instruction) PrettyPrint(_ []Node) string {
	out := "\t" + ins.OpCode.Name
	paramStrs := make([]string, 0, len(ins.Params))
	for _, param := range ins.Params {
		paramStrs = append(paramStrs, param.String())
	}
	out += "\t" + strings.Join(paramStrs, string(op.SeparatorChar)+" ")
	return out
}

func (ins Instruction) String() string {
	out := "<" + ins.OpCode.Name
	paramStrs := make([]string, 0, len(ins.Params))
	for _, param := range ins.Params {
		paramStrs = append(paramStrs, param.String())
	}
	if len(paramStrs) == 0 {
		return out + ">"
	}
	out += " (" + strings.Join(paramStrs, string(op.SeparatorChar)+" ") + ")"
	return out + ">"
}

func (ins Instruction) ValidateParameters() error {
	if len(ins.Params) != len(ins.OpCode.ParamTypes) {
		return fmt.Errorf("expected %d parameters, got %d", len(ins.OpCode.ParamTypes), len(ins.Params))
	}
	for i, param := range ins.Params {
		// Check that `param.Typ` bytes is within the ins.OpCode.ParamTypes[i] mask.
		if param.Typ&ins.OpCode.ParamTypes[i] == 0 {
			return fmt.Errorf("invalid parameter %d type %q for %q, expect %q", i+1, param.Typ, ins.OpCode.Name, ins.OpCode.ParamTypes[i])
		}
	}

	for _, param := range ins.Params {
		if param.Value == "" {
			return fmt.Errorf("parameter value cannot be empty")
		}
	}

	return nil
}

func (ins Instruction) Encode(p *Program) error {
	// Has labels are indexed from the instruction start
	// keep track of it.
	idxInstruction := p.idx

	// Store the opcode and advance.
	p.buf[p.idx] = ins.OpCode.Code
	p.idx++
	// If the instruction requires an encoding byte,
	// store it and advance.
	if ins.OpCode.EncodingByte {
		p.buf[p.idx] = ins.ParamsEncoding()
		p.idx++
	}
	// Encode each param.
	for _, param := range ins.Params {
		// Handle case for label references.
		if strings.HasPrefix(param.Value, string(op.LabelChar)) {
			// If we already know the label, we can
			// replace the it with mdmdhe offset.
			// Otherwise, keep going, it will be known the second time.
			if _, ok := p.labels[param.Value[1:]]; ok {
				param.Value = strconv.Itoa(p.labels[param.Value[1:]] - idxInstruction)
			} else if !p.hasLabelIndex {
				p.hasMissingLabels = true
			} else {
				// If we don't know the label while having
				// the labels index, error out.
				return fmt.Errorf("unknown label %q", param.Value)
			}
		}
		// Encode the paramter and advance the index.
		n, err := param.Encode(p.buf[p.idx:], ins.OpCode.ParamMode)
		if err != nil {
			return fmt.Errorf("failed to encode parameter %s: %w", param, err)
		}
		p.idx += n
	}

	return nil
}
