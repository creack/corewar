package main

const (
	MemSize       = 6 * 1024
	IdxMod        = 512 // Index modulo. // TODO: What?
	MaxArgsNumber = 4   // This may not be changed 2^*IND_SIZE // TODO: What?
)

const (
	RegisterCount = 16 // r1 <--> r16
)

// InstructionType enum type.
type InstructionType int

// InstructionType values.
const (
	TReg InstructionType = 1 << iota // Register.
	TDir                             // Direct.
	TInd                             // Indirect. Relative. (ld 1,r1 puts what is at address (1+pc) in r1 (4 bytes)).
)

func (it InstructionType) Size() int {
	switch it {
	case TReg:
		return RegisterSize
	case TDir:
		return DirectSize
	case TInd:
		return IndirectSize
	default:
		return -1
	}
}

// Tokens.
const (
	CommentChar      = '#'
	LabelChar        = ':'
	DirectChar       = '%'
	SeparatorChar    = ','
	LabelChars       = "abcdefghijklmnopqrstuvwxyz_0123456789"
	NameCmdString    = ".name"
	CommentCmdString = ".comment"
)

// InstructionDef is the definition of instructions.
type InstructionDef struct {
	Name    string
	Code    int
	Types   []InstructionType
	Cycles  int
	Comment string
}

var InstructionTable = []InstructionDef{
	{"live", 0x01, []InstructionType{TDir}, 10, "alive"},
	{"ld", 0x02, []InstructionType{TDir | TInd, TReg}, 5, "load"},
	{"st", 0x03, []InstructionType{TReg, TInd | TReg}, 5, "store"},
	{"add", 0x04, []InstructionType{TReg, TReg, TReg}, 10, "addition"},
	{"sub", 0x05, []InstructionType{TReg, TReg, TReg}, 10, "subtraction"},
	{"and", 0x06, []InstructionType{TReg | TDir | TInd, TReg | TInd | TDir, TReg}, 6, "and  r1,r2,r3   r1&r2 -> r3"},
	{"or", 0x07, []InstructionType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 6, "or   r1,r2,r3   r1|r2 -> r3"},
	{"xor", 0x08, []InstructionType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 6, "xor  r1,r2,r3   r1^r2 -> r3"},
	{"zjmp", 0x09, []InstructionType{TDir}, 20, "jump if zero"},
	{"ldi", 0x0a, []InstructionType{TReg | TDir | TInd, TDir | TReg, TReg}, 25, "load index"},
	{"sti", 0x0b, []InstructionType{TReg, TReg | TDir | TInd, TDir | TReg}, 25, "store index"},
	{"fork", 0x0c, []InstructionType{TDir}, 800, "fork"},
	{"lld", 0x0d, []InstructionType{TDir | TInd, TReg}, 10, "long load"},
	{"lldi", 0x0e, []InstructionType{TReg | TDir | TInd, TDir | TReg, TReg}, 50, "long load index"},
	{"lfork", 0x0f, []InstructionType{TDir}, 1000, "long fork"},
	{"aff", 0x10, []InstructionType{TReg}, 2, "aff"},
}

// Sizes in bytes.
const (
	RegisterSize = 1
	IndirectSize = 2
	DirectSize   = 4
)

// Header.
const (
	ProgNameLength   = 128
	CommentLength    = 2048
	CorewarExecMagic = 0xea83f3 // Why not?
)

type ChampionHeader struct {
	Magic    int
	ProgName [ProgNameLength + 1]byte
	ProgSize int
	Comment  [CommentLength + 1]byte
}

// VM settings.
const (
	CyclesToDie = 1536 // Number of cycles to be declared dead.
	CycleDelay  = 5
	NumLives    = 40
)
