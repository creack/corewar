package op

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
