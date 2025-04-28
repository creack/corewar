package vm

type MessageType int

const (
	_ MessageType = iota
	MsgDebug
	MsgError
	MsgWarning
	MsgDisplay
	MsgLive
	MsgLiveMiss
	MsgDead
	MsgGameOver
	MsgClear
	MsgPause
	MsgDump
)

func (mt MessageType) String() string {
	switch mt {
	case MsgDebug:
		return "Debug"
	case MsgError:
		return "Error"
	case MsgWarning:
		return "Warning"
	case MsgDisplay:
		return "Display"
	case MsgLive:
		return "Live"
	case MsgLiveMiss:
		return "Live Miss"
	case MsgDead:
		return "Dead"
	case MsgGameOver:
		return "Game Over"
	case MsgClear:
		return "Clear"
	case MsgPause:
		return "Pause"
	case MsgDump:
		return "Dump"
	default:
		return "Unknown"
	}
}

type Message struct {
	Type    MessageType
	Process *Process
	Message string
}

func NewMessage(mt MessageType, p *Process, msg string) Message {
	return Message{
		Type:    mt,
		Process: p,
		Message: msg,
	}
}
