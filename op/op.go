package op

import (
	"encoding/binary"
	"strings"
)

var Endian = binary.BigEndian

const (
	MemSize       = 6 * 1024 // 6Kb.
	IdxMod        = 512      // Index modulo.
	MaxArgsNumber = 4        // This may not be changed. Arbitrary rule.
)

const (
	RegisterCount = 16 // r1 <--> r16
	RegisterSize  = 4  // Size of each register in bytes.
)

// ParamType enum type.
type ParamType int

// InstructionType values.
const (
	TReg ParamType = 1 << iota // Register.
	TDir                       // Direct.
	TInd                       // Indirect. Relative. (ld 1,r1 puts what is at address (1+pc) in r1 (4 bytes)).
	TLab                       // Label. Unused.
)

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
	// Param sizes in bytes.
	const (
		Register = 1
		Indirect = 2 // Indirect/Index.
		Direct   = 4
	)
	switch pt {
	case TReg:
		return Register
	case TDir:
		return Direct
	case TInd:
		return Indirect
	default:
		return -1
	}
}

// Tokens.
const (
	CommentChars     = "#;"
	LabelChar        = ':'
	DirectChar       = '%'
	RegisterChar     = 'r'
	SeparatorChar    = ','
	DirectiveChar    = '.'
	LabelChars       = "abcdefghijklmnopqrstuvwxyz_0123456789"
	RawCodeChars     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"
	NameCmdString    = ".name"
	CommentCmdString = ".comment"
)

type ParamMode int

const (
	ParamModeDynamic ParamMode = iota // Encoded based on the opcode parameter type.
	ParamModeValue                    // Always encoded as Direct value.
	ParamModeIndex                    // Always encoded as Indirect (index) value.
)

func (am ParamMode) String() string {
	switch am {
	case ParamModeDynamic:
		return "dynamic"
	case ParamModeValue:
		return "value"
	case ParamModeIndex:
		return "index"
	default:
		return "unknown param mode"
	}
}

// OpCode is the definition of instructions.
type OpCode struct {
	Name         string
	ParamTypes   []ParamType
	Code         byte
	Cycles       int
	Comment      string
	ParamMode    ParamMode
	EncodingByte bool
	SetCarry     bool
}

var OpCodeTable = []OpCode{
	{"noop", nil, 0, 0, "noop", 0, false, false},
	{"live", []ParamType{TDir}, 1, 10, "alive", 0, false, false},
	{"ld", []ParamType{TDir | TInd, TReg}, 2, 5, "load", 0, true, true},
	{"st", []ParamType{TReg, TInd | TReg}, 3, 5, "store", ParamModeIndex, true, false},
	{"add", []ParamType{TReg, TReg, TReg}, 4, 10, "addition", ParamModeValue, true, true},
	{"sub", []ParamType{TReg, TReg, TReg}, 5, 10, "subtraction", ParamModeValue, true, true},
	{"and", []ParamType{TReg | TDir | TInd, TReg | TInd | TDir, TReg}, 6, 6, "and  r1,r2,r3   r1&r2 -> r3", ParamModeValue, true, true},
	{"or", []ParamType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 7, 6, "or   r1,r2,r3   r1|r2 -> r3", ParamModeValue, true, true},
	{"xor", []ParamType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 8, 6, "xor  r1,r2,r3   r1^r2 -> r3", ParamModeValue, true, true},
	{"zjmp", []ParamType{TDir}, 9, 20, "jump if zero", ParamModeIndex, false, false},
	{"ldi", []ParamType{TReg | TDir | TInd, TDir | TReg, TReg}, 10, 25, "load index", ParamModeIndex, true, true},
	{"sti", []ParamType{TReg, TReg | TDir | TInd, TDir | TReg}, 11, 25, "store index", ParamModeIndex, true, false},
	{"fork", []ParamType{TDir}, 12, 800, "fork", ParamModeIndex, false, false},
	{"lld", []ParamType{TDir | TInd, TReg}, 13, 10, "long load", ParamModeValue, true, true},
	{"lldi", []ParamType{TReg | TDir | TInd, TDir | TReg, TReg}, 14, 50, "long load index", ParamModeIndex, true, true},
	{"lfork", []ParamType{TDir}, 15, 1000, "long fork", ParamModeIndex, false, true},
	{"aff", []ParamType{TReg}, 16, 2, "display reg", ParamModeDynamic, true, false},
}

// Header.
const (
	ProgNameLength   = 128
	CommentLength    = 2048
	CorewarExecMagic = 0xea83f3 // Why not?
)

type ChampionHeader struct {
	Magic    uint32
	ProgName [ProgNameLength + 1]byte
	ProgSize uint32
	Comment  [CommentLength + 1]byte
}

// VM settings.
const (
	CyclesToDie = 1536 // Number of cycles to be declared dead.
	CycleDelay  = 5
	NumLives    = 40
)
