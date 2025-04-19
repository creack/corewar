package op

import "strings"

// ParamType enum type.
type ParamType int

func (pt ParamType) Encoding() byte {
	switch pt {
	case TReg:
		return 0b01
	case TDir:
		return 0b10
	case TInd, TLab:
		return 0b11
	default:
		return 0
	}
}

func (ParamType) Decoding(b byte) ParamType {
	switch b {
	case 0b01:
		return TReg
	case 0b10:
		return TDir
	case 0b11:
		return TInd
	default:
		return 0
	}
}

func (pt ParamType) String() string {
	var parts []string
	if pt&TReg != 0 {
		parts = append(parts, "register")
	}
	if pt&TDir != 0 {
		parts = append(parts, "direct")
	}
	if pt&TInd != 0 {
		parts = append(parts, "indirect")
	}
	if pt&TLab != 0 {
		parts = append(parts, "label")
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "|")
}

func (pt ParamType) Size() int {
	switch pt {
	case TReg:
		// Registers are 4 bytes, but register params are on 1 byte.
		return 1
	case TDir:
		return DirectSize
	case TInd:
		return IndirectSize
	default:
		return -1
	}
}
