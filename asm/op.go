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
	TReg   InstructionType = 1 << iota // Register.
	TDir                               // Direct.
	TInd                               // Indirect. Relative. (ld 1,r1 puts what is at address (1+pc) in r1 (4 bytes)).
	TLabel                             // Label.
)

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

// Instructions.
type Instruction struct {
	Name    string
	Types   []InstructionType
	Cycles  int
	Comment string
}

var InstructionTable = []Instruction{
	{}, // No-op as instructions index starts at 1.
	{"live", []InstructionType{TDir}, 10, "alive"},
	{"ld", []InstructionType{TDir | TInd, TReg}, 5, "load"},
	{"st", []InstructionType{TReg, TInd | TReg}, 5, "store"},
	{"add", []InstructionType{TReg, TReg, TReg}, 10, "addition"},
	{"sub", []InstructionType{TReg, TReg, TReg}, 10, "subtraction"},
	{"and", []InstructionType{TReg | TDir | TInd, TReg | TInd | TDir, TReg}, 6, "and  r1,r2,r3   r1&r2 -> r3"},
	{"or", []InstructionType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 6, "or   r1,r2,r3   r1|r2 -> r3"},
	{"xor", []InstructionType{TReg | TInd | TDir, TReg | TInd | TDir, TReg}, 6, "xor  r1,r2,r3   r1^r2 -> r3"},
	{"zjmp", []InstructionType{TDir}, 20, "jump if zero"},
	{"ldi", []InstructionType{TReg | TDir | TInd, TDir | TReg, TReg}, 25, "load index"},
	{"sti", []InstructionType{TReg, TReg | TDir | TInd, TDir | TReg}, 25, "store index"},
	{"fork", []InstructionType{TDir}, 800, "fork"},
	{"lld", []InstructionType{TDir | TInd, TReg}, 10, "long load"},
	{"lldi", []InstructionType{TReg | TDir | TInd, TDir | TReg, TReg}, 50, "long load index"},
	{"lfork", []InstructionType{TDir}, 1000, "long fork"},
	{"aff", []InstructionType{TReg}, 2, "aff"},
}

// Sizes in bytes.
const (
	IndirectSize int = 2
	RegisterSize int = 4
	DirectSize       = RegisterSize
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
