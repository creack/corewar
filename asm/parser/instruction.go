package parser

import (
	"fmt"
	"strconv"
	"strings"

	"go.creack.net/corewar/op"
)

type Instruction struct {
	OpCode op.OpCode    // OpCode reference.
	Params []*Parameter // Parameters.
	Size   int          // In bytes, only set when decoding.
}

func (ins *Instruction) ParamsDecoding(b byte) error {
	// If the opcode doesn't have an encoding byte,
	// use the type from the definition.
	if !ins.OpCode.EncodingByte {
		for _, elem := range ins.OpCode.ParamTypes {
			ins.Params = append(ins.Params, &Parameter{Typ: elem})
		}
		return nil
	}

	// Reverse the process of the encoding byte.
	for i, validTypes := range ins.OpCode.ParamTypes {
		t := new(op.ParamType).Decoding((b >> byte((3-i)*2)) & 0b11)
		if t == 0 {
			return fmt.Errorf("invalid encoding byte %x for %q", b, ins.OpCode.Name)
		}

		if t&validTypes == 0 {
			return fmt.Errorf("invalid parameter %d type %q for %q, expected %q", i+1, t, ins.OpCode.Name, validTypes)
		}
		ins.Params = append(ins.Params, &Parameter{Typ: t})
	}

	return nil
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
	return fmt.Sprintf("%- 8s %s", out, strings.Join(paramStrs, string(op.SeparatorChar)+" "))
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
		if param.RawValue == "" {
			return fmt.Errorf("parameter value cannot be empty")
		}
	}

	return nil
}

func (ins Instruction) Encode(p *Program) ([]byte, error) {
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
		if param.Resolved == "" && strings.HasPrefix(param.RawValue, string(op.LabelChar)) {
			// If we already know the label, we can
			// replace the it with mdmdhe offset.
			// Otherwise, keep going, it will be known the second time.
			if _, ok := p.labels[param.RawValue[1:]]; ok {
				param.Value = int64(p.labels[param.RawValue[1:]] - idxInstruction)
				param.Resolved = strconv.Itoa(int(param.Value))
			} else if !p.hasLabelIndex {
				p.hasMissingLabels = true
			} else {
				// If we don't know the label while having
				// the labels index, error out.
				return nil, fmt.Errorf("unknown label %q", param.RawValue)
			}
		}
		for i, elem := range param.Modifiers {
			if elem.resolved != "" || !strings.HasPrefix(elem.raw, string(op.LabelChar)) {
				continue
			}
			if _, ok := p.labels[elem.raw[1:]]; ok {
				param.Modifiers[i].resolved = strconv.Itoa(p.labels[elem.raw[1:]] - idxInstruction)
			} else if !p.hasLabelIndex {
				p.hasMissingLabels = true
			} else {
				// If we don't know the label while having
				// the labels index, error out.
				return nil, fmt.Errorf("unknown label %q", elem)
			}
		}

		// Encode the paramter and advance the index.
		n, err := param.Encode(p.buf[p.idx:], ins.OpCode.ParamMode, p.strict)
		if err != nil {
			return nil, fmt.Errorf("failed to encode parameter %s: %w", param, err)
		}
		p.idx += n
	}

	return p.buf[idxInstruction:p.idx], nil
}

var ErrInvalidOpcode = fmt.Errorf("invalid opcode")

// Decodes the next instruction if valid. Returns the instruction and the
// how many bytes have been consumed.
func DecodeNextInstruction(buf []byte) (*Instruction, int, error) {
	idx := 0

	if len(buf) == 0 {
		return nil, idx, fmt.Errorf("empty buffer")
	}

	b := buf[idx]
	if int(b) >= len(op.OpCodeTable) {
		return nil, idx, fmt.Errorf("invalid instruction %x: %w", b, ErrInvalidOpcode)
	}

	idx++
	if idx >= len(buf) {
		return nil, idx, fmt.Errorf("invalid instruction, missing encoding and/or parameters")
	}
	ins := &Instruction{
		OpCode: op.OpCodeTable[b],
	}

	var encodingByte byte
	if ins.OpCode.EncodingByte {
		encodingByte = buf[idx]
		idx++
		if idx >= len(buf) {
			return nil, idx, fmt.Errorf("invalid instruction, missing parameters after encoding")
		}
	}
	// Even if we don't have the endoding byte, call the paramsdecoding method.
	// Will populate the params with the opcode types if needed.
	if err := ins.ParamsDecoding(encodingByte); err != nil {
		return nil, idx, fmt.Errorf("failed to decode instruction: %w", err)
	}

	for _, elem := range ins.Params {
		if idx >= len(buf) {
			return nil, idx, fmt.Errorf("invalid instruction, missing parameter data")
		}
		// Registers are always 1 byte.
		if elem.Typ == op.TReg {
			elem.Value = int64(buf[idx])
			elem.RawValue = fmt.Sprintf("%d", buf[idx])
			idx++
			continue
		}
		// In dynamic mode, we need to check the type of the parameter.
		if ins.OpCode.ParamMode == op.ParamModeDynamic {
			if elem.Typ == op.TDir {
				// Direct value, we need to read 4 bytes.
				elem.Value = int64(op.Endian.Uint32(buf[idx : idx+4]))
				elem.RawValue = fmt.Sprintf("%d", uint32(elem.Value))
				idx += 4
			} else if elem.Typ == op.TInd {
				// Indirect value, we need to read 2 bytes.
				elem.Value = int64(op.Endian.Uint16(buf[idx : idx+2]))
				elem.RawValue = fmt.Sprintf("%d", uint16(elem.Value))
				idx += 2
			} else {
				return nil, idx, fmt.Errorf("invalid parameter type %q for %q", elem.Typ, ins.OpCode.Name)
			}
			continue
		}
		if ins.OpCode.ParamMode == op.ParamModeIndex {
			// Always an indirect value, we need to read 2 bytes.
			elem.Value = int64(op.Endian.Uint16(buf[idx : idx+2]))
			elem.RawValue = fmt.Sprintf("%d", int16(elem.Value))
			idx += 2
			continue
		}
		return nil, idx, fmt.Errorf("invalid param mode %q for %q", ins.OpCode.ParamMode, ins.OpCode.Name)
	}
	ins.Size = idx
	return ins, idx, nil
}
