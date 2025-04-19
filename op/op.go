package op

import (
	"encoding/binary"
	"strings"
)

var Endian = binary.BigEndian

const (
	MemSize       = 6 * 1024 // 6Kb.
	IdxMod        = 512      // Index modulo.
	MaxArgsNumber = 4        // This may not be changed. Arbitrary rule. // TODO: Add validation for this.
)

const (
	RegisterCount = 16 // r1 <--> r16
	RegisterSize  = 4  // Size of each register in bytes. Hard-coded to uint32. Can't be changed.
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
	ParamModeIndex                    // Always encoded as index (2 bytes) even for direct parameters.
)

func (am ParamMode) String() string {
	switch am {
	case ParamModeDynamic:
		return "dynamic"
	case ParamModeIndex:
		return "index"
	default:
		return "unknown param mode"
	}
}

// OpCode is the definition of instructions.
type OpCode struct {
	// Original fields.
	Name       string
	ParamTypes []ParamType
	Code       byte
	Cycles     int
	Comment    string

	// Added for ease of use.
	ParamMode    ParamMode
	EncodingByte bool
}

var OpCodeTable = []OpCode{
	{"noop", nil, 0, 0, "noop", 0, false},
	{"live", []ParamType{TDir}, 1, 10, "alive", 0, false},
	{"ld", []ParamType{TDir | TInd, TReg}, 2, 5, "load", 0, true},
	{"st", []ParamType{TReg, TInd | TReg}, 3, 5, "store", 0, true},
	{"add", []ParamType{TReg, TReg, TReg}, 4, 10, "addition", 0, true},
	{"sub", []ParamType{TReg, TReg, TReg}, 5, 10, "subtraction", 0, true},
	{"and", []ParamType{TReg | TDir | TInd, TReg | TInd | TDir, TReg}, 6, 6, "and  r1,r2,r3   r1&r2 -> r3", 0, true},
	{"or", []ParamType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 7, 6, "or   r1,r2,r3   r1|r2 -> r3", 0, true},
	{"xor", []ParamType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 8, 6, "xor  r1,r2,r3   r1^r2 -> r3", 0, true},
	{"zjmp", []ParamType{TDir}, 9, 20, "jump if zero", ParamModeIndex, false},
	{"ldi", []ParamType{TReg | TDir | TInd, TDir | TReg, TReg}, 10, 25, "load index", ParamModeIndex, true},
	{"sti", []ParamType{TReg, TReg | TDir | TInd, TDir | TReg}, 11, 25, "store index", ParamModeIndex, true},
	{"fork", []ParamType{TDir}, 12, 800, "fork", ParamModeIndex, false},
	{"lld", []ParamType{TDir | TInd, TReg}, 13, 10, "long load", 0, true},
	{"lldi", []ParamType{TReg | TDir | TInd, TDir | TReg, TReg}, 14, 50, "long load index", ParamModeIndex, true},
	{"lfork", []ParamType{TDir}, 15, 1000, "long fork", ParamModeIndex, false},
	{"aff", []ParamType{TReg}, 16, 2, "display reg", 0, true},
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

// StructSize returns the size of the header struct.
// Similar to unsafe.Sizeof, but allow disabling alignment,
// and use hardcoded values instead of the dynamic ones based
// on the current system/architecture.
// Return the full size, the size of the name and comment fields.
func (h ChampionHeader) StructSize() (headerSize, nameLength, commentLength int) {
	align := 4      // Align on 4 bytes.
	headerSize += 4 // magic number.

	nameLength = ProgNameLength + 1
	if n := nameLength % align; n != 0 {
		nameLength += (align - n)
	}
	headerSize += nameLength

	headerSize += 4 // prog size.

	commentLength = CommentLength + 1
	if n := commentLength % align; n != 0 {
		commentLength += (align - n)
	}
	headerSize += commentLength

	return headerSize, nameLength, commentLength
}

// VM settings.
const (
	CyclesToDie = 1536 // Number of cycles to be declared dead.
	NumLives    = 40   // Number of 'live' calls before updating CyclesToDie.
	CycleDelta  = 5    // Number of cycles to be remove from CyclesToDie after NumLives.
)
