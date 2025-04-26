package op

const (
	MemSize       = 4 * 1024    // Memory size in bytes.
	IdxMod        = MemSize / 8 // Index modulo, i.e. how far can a player go in the memory (except for long instructions).
	MaxArgsNumber = 4           // This may not be changed. Arbitrary rule. // TODO: Add validation for this.
)

// Lexer Tokens.
const (
	CommentChars  = "#;"
	LabelChar     = ':'
	DirectChar    = '%'
	RegisterChar  = 'r'
	SeparatorChar = ','
	DirectiveChar = '.'
	LabelChars    = "abcdefghijklmnopqrstuvwxyz_0123456789"
	RawCodeChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"
)

// Avaialble directives.
const (
	NameCmdString    = ".name"
	CommentCmdString = ".comment"
	ExtendCmdString  = ".extend"
	CodeCmdString    = ".code"
)

const (
	RegisterCount = 16 // r1 <--> r16
)

// ParamType enum type.
type ParamType byte

// InstructionType values.
const (
	TReg ParamType = 1 << iota // Register.
	TDir                       // Direct.
	TInd                       // Indirect. Relative. (ld 1,r1 puts what is at address (1+pc) in r1 (4 bytes)).
	TLab                       // Label. Unused.
)

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

// Sizes in bytes.
const (
	IndirectSize = 2 // Indirect/Index.
	RegisterSize = 4 // Size of each register. Hard-coded to uint32. Can't be changed.
	DirectSize   = RegisterSize
)

// Header.
const (
	ProgNameLength   = 128
	CommentLength    = 2048
	CorewarExecMagic = 0xea83f3 // Why not?
)

type Header struct {
	Magic    uint32
	ProgName [ProgNameLength + 1]byte
	ProgSize uint32
	Comment  [CommentLength + 1]byte
}

// VM settings.
const (
	CyclesToDie = 1536 // Number of cycles to be declared dead.
	CycleDelta  = 50   // Number of cycles to be remove from CyclesToDie after NumLives.
	NumLives    = 21   // Number of 'live' calls before updating CyclesToDie.
)
