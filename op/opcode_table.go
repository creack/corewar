package op

var OpCodeTable = []OpCode{
	{"noop", nil, 0, 1, "noop", 0, false},
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
