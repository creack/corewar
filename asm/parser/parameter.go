package parser

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"go.creack.net/corewar/op"
)

type Modifier struct {
	raw      string
	resolved string
	n        int64
}

// Parameter represents a parameter in an instruction.
// Stored as raw string.
type Parameter struct {
	Typ       op.ParamType
	Value     int64
	RawValue  string
	Resolved  string
	Modifiers []Modifier // Modifiers for the parameter.
}

func (p Parameter) String() string {
	var out string
	switch p.Typ {
	case op.TReg:
		out = string(op.RegisterChar) + p.RawValue
	case op.TInd:
		out = p.RawValue
	case op.TDir:
		out = string(op.DirectChar) + p.RawValue
	case op.TLab:
		out = string(op.LabelChar) + p.RawValue
	default:
		return fmt.Sprintf("unknown param type %d", p.Typ)
	}
	for _, elem := range p.Modifiers {
		out += elem.raw
	}
	return out
}

// NOTE: Some champions like 42.sh have numbers overflowing 32bits.
func parseNumber(in string) (int64, error) {
	if strings.HasPrefix(in, "0x") || strings.HasPrefix(in, "0X") {
		return strconv.ParseInt(in[2:], 16, 64)
	} else if strings.HasPrefix(in, "0o") || strings.HasPrefix(in, "0O") {
		return strconv.ParseInt(in[2:], 8, 64)
	} else if strings.HasPrefix(in, "0b") || strings.HasPrefix(in, "0B") {
		return strconv.ParseInt(in[2:], 2, 64)
	}
	return strconv.ParseInt(in, 10, 64)
}

// Encode the parameter in the given buffer based
// on the paramter type and param mode (from the instruction opcode).
// If the given buffer is nil, it will not write anything but still return
// how many bytes would have been written.
func (p Parameter) Encode(buf []byte, paramMode op.ParamMode, strict bool) (int, error) {
	val := p.Resolved
	if val == "" {
		val = p.RawValue
	}
	// Parse the value as a number.
	n, err := parseNumber(val)
	p.Value = int64(n)
	if err != nil && !strings.HasPrefix(val, string(op.LabelChar)) {
		// If we have a label, it will error out but keep going. Will pass next time around.
		return 0, fmt.Errorf("parse %q: %w", val, err)
	}

	// Apply modifiers if any.
	// TODO: Handle the case where we have an operator at the end with a missing number.
	for _, elem := range p.Modifiers {
		elemVal := elem.resolved
		if elemVal == "" {
			elemVal = elem.raw
		}
		// If we still have a label reference unresolved,
		// we can't calculate the value yet. Break away,
		// next iteration will handle it.
		if strings.HasPrefix(elemVal, string(op.LabelChar)) && elem.resolved == "" {
			break
		}
		var neg bool
		switch elem.raw {
		case "-":
			neg = true
		case "+":
		default:
			n1, err := parseNumber(elemVal)
			if err != nil {
				return 0, fmt.Errorf("parse modifier %q: %w", elemVal, err)
			}
			if neg {
				n1 *= -1
			}
			n += n1
		}
	}

	// Simplest case, register.
	if p.Typ == op.TReg {
		// NOTE: Registers only go from 1 to op.RegisterCount (16),
		// when in strict mode, enforce it, otherwise, let it go.
		if n < 1 || n > op.RegisterCount {
			if strict {
				return 0, fmt.Errorf("invalid register number %d", n)
			}
			log.Printf("Warning: invalid register number %d for parameter %s", n, p)
		}
		if buf != nil {
			buf[0] = byte(n)
		}
		return 1, nil
	}

	// If the param mode is dynamic, we need to check the type.
	if paramMode == op.ParamModeDynamic {
		if p.Typ == op.TInd {
			if buf != nil {
				op.Endian.PutUint16(buf, uint16(n))
			}
		} else {
			if buf != nil {
				op.Endian.PutUint32(buf, uint32(n))
			}
		}
		return p.Typ.Size(), nil
	}

	// Handle when the param mode is fixed by the opcode.
	switch paramMode {
	case op.ParamModeIndex:
		if buf != nil {
			op.Endian.PutUint16(buf, uint16(n))
		}
		return 2, nil
	default:
		return 0, fmt.Errorf("unexpected param mode %q for parameter %q", paramMode, p)
	}
}
