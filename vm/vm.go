// Package main is the entry point of the program.
package vm

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"

	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/op"
)

type Process struct {
	ID     int
	Player *Player

	Registers      [op.RegisterCount]uint32
	PC             uint32
	Carry          bool
	CurInstruction *parser.Instruction
	WaitCycles     int
}

type Player struct {
	Name   string
	Number int
	Dead   bool

	ProcessCount int // Number of processes.

	TotalLives   int // Total number of 'live' calls.'
	CurrentLives int // Number of 'live' calls in the current CyclesToDie window.
}

type PlayerConfig struct {
	Number int
	Data   []byte
}

type Config struct {
	MemSize     int // Size of the memory.
	IdxMod      int // Index modulo, i.e. how far can a player go in the memory (except for long instructions).
	CyclesToDie int // Window where players need to say they are alive.
	CycleDelta  int // How many cycles to remove from CyclesToDie NumLives is reached.
	NumLives    int // Number of 'live' calls before updating CyclesToDie.

	Players []PlayerConfig
}

type Corewar struct {
	Config Config

	Ram Ram

	Players   []*Player
	Processes []*Process
	NextPID   int

	Cycle          int // Current cycle.
	CurCyclesToDie int // How many cycles until death players have after each 'live' call.
	LiveCalls      int // Number of 'live' calls since last check.

	// Messages is a channel where the VM will send messages.
	// Needs to be consumed otherwise it will block.
	Messages chan Message `json:"-"`
}

// NextCycle advances the cycle counter until a player is ready to go
// or cycleToDie expires.
// Useful when everyone is waiting for a long instruction like fork.
func (cw *Corewar) NextCycle() {
	cycles := cw.CurCyclesToDie

	// Then check in how many cycles the next process
	// instruction is ready to execute.
	for _, p := range cw.Processes {
		if p.CurInstruction != nil {
			cycles = min(cycles, p.WaitCycles)
		}
	}

	// Make sure we always advance at least one cycle.
	if cycles <= 1 {
		cycles = 1
	}

	// Update the cycle count.
	cw.Cycle += cycles
	cw.CurCyclesToDie -= cycles

	// Decrement the wait cycles for each process.
	for _, p := range cw.Processes {
		if p.CurInstruction != nil {
			p.WaitCycles -= cycles
		}
	}
}

// func (cw *Corewar) GetRamValue32(addr uint32) uint32 {
// 	return op.Endian.Uint32(cw.Ram[addr%uint32(len(cw.Ram)):])
// }

// func (cw *Corewar) GetRamValueIndex(pc uint32, idx int16, mod int64) uint16 {
// 	return op.Endian.Uint16(cw.Ram[(int64(pc)+(int64(idx)%mod))%int64(len(cw.Ram)):])
// }

// func (cw *Corewar) SetRamValue(addr uint32, value uint32) {
// 	op.Endian.PutUint32(cw.Ram[addr%uint32(len(cw.Ram)):], value)
// }

func opAdd(a, b int64) int64 { return a + b }
func opSub(a, b int64) int64 { return a - b }
func opAnd(a, b int64) int64 { return a & b }
func opOr(a, b int64) int64  { return a | b }
func opXor(a, b int64) int64 { return a ^ b }

// mathOp returns the op function for common math operations.
func mathOp(operation func(a, b int64) int64) func(cw *Corewar, p *Process) bool {
	return func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		var source1, source2 int64
		target := ins.Params[2].Value - 1

		if ins.Params[0].Typ == op.TReg {
			source1 = int64(p.Registers[ins.Params[0].Value-1])
		} else if ins.Params[0].Typ == op.TDir {
			source1 = int64(ins.Params[0].Value)
		} else {
			source1 = int64(cw.Ram.GetRamValue32(p, uint32(int64(p.PC)+int64(ins.Params[0].Value)%int64(cw.Config.IdxMod))))
		}

		if ins.Params[1].Typ == op.TReg {
			source2 = int64(p.Registers[ins.Params[1].Value-1])
		} else if ins.Params[1].Typ == op.TDir {
			source2 = int64(ins.Params[1].Value)
		} else {
			source2 = int64(cw.Ram.GetRamValue32(p, uint32(int64(p.PC)+int64(ins.Params[1].Value)%int64(cw.Config.IdxMod))))
		}

		p.Registers[target] = uint32(operation(source1, source2))

		p.Carry = p.Registers[target] == 0
		return true
	}
}

var ops = func() map[int]func(cw *Corewar, p *Process) bool {
	ops := map[int]func(cw *Corewar, p *Process) bool{}

	// noop.
	ops[0x00] = func(cw *Corewar, p *Process) bool { return true }

	// live. Declare a player alive.
	// 1 Param, always a direct value.
	ops[0x01] = func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		cw.LiveCalls++ // Global live count increases event if the target player is invalid/dead.
		i := slices.IndexFunc(cw.Players, func(p *Player) bool { return p.Number == int(ins.Params[0].Value) })
		if i == -1 || i >= len(cw.Players) || cw.Players[i].Dead {
			cw.Messages <- NewMessage(MsgLiveMiss, p, fmt.Sprintf("Missed 'live' from %d (%s)", p.Player.Number, p.Player.Name))
			return true
		}
		targetPlayer := cw.Players[i]
		targetPlayer.TotalLives++
		targetPlayer.CurrentLives++
		cw.Messages <- NewMessage(MsgLive, p, fmt.Sprintf("Player %d (%s) is alive", targetPlayer.Number, targetPlayer.Name))
		return true
	}

	// ld. Loads the value of the first param into the second.
	// Updates the carry.
	// 2 Params:
	// - 1: Can be a direct value or an indirect value.
	// - 2: Always a register, update the content.
	ops[0x02] = func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		mod := int64(cw.Config.IdxMod)
		// If the instruction is lld, we don't use the modulo.
		if ins.OpCode.Code == 0x0d {
			mod = 1
		}

		// Target register.
		r := ins.Params[1].Value - 1

		// If the first param is a Direct value, use it directly.
		if ins.Params[0].Typ == op.TDir {
			p.Registers[r] = uint32(ins.Params[0].Value)
			cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("LD Direct 0x%04x into R%d", ins.Params[0].Value, r))
		} else {
			// If the first param is an indirect value, we need to
			// read the value from the RAM.
			// - `ld 34,r3` loads the REG_SIZE bytes starting at the address PC + 34 % IDX_MOD into r3.
			p.Registers[r] = cw.Ram.GetRamValue32(p, uint32(int64(p.PC)+int64(ins.Params[0].Value)%mod))
			cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("LD RAM %d (0x%04x) into R%d", uint32(int64(ins.Params[0].Value)%mod), p.Registers[r], r))
		}

		// Update the carry.
		p.Carry = p.Registers[r] == 0

		return true
	}

	// st. Store the 1st param into the 2nd.
	// 2 Params:
	// - 1: Always a register, take the content.
	// - 2: Can be a target register or an indirect value.
	ops[0x03] = func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		// Source: register content.
		source := p.Registers[ins.Params[0].Value-1]

		// If the target is a register, we replace its value.
		// - `st r2,r8` copies the content of r3 into r8.
		if ins.Params[1].Typ == op.TReg {
			p.Registers[ins.Params[1].Value-1] = source
			if ins.Params[0].Typ == op.TReg {
				cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("ST R%d (0x%04x) into R%d", ins.Params[0].Value, source, ins.Params[1].Value-1))
			} else {
				cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("ST RAM %d (0x%04x) into R%d", ins.Params[0].Value, source, ins.Params[1].Value-1))
			}
			return true
		}

		// If the target is an indirect value, we store the content of the
		// source register into the RAM.
		// - `st r4,34` stores the content of r4 at the address PC + 34 % IDX_MOD.
		cw.Ram.SetRamValue(p, uint32(int64(p.PC)+ins.Params[1].Value%int64(cw.Config.IdxMod)), source)
		if ins.Params[0].Typ == op.TReg {
			cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("ST R%d (0x%04x) into RAM %d", ins.Params[0].Value, source, ins.Params[1].Value%int64(cw.Config.IdxMod)))
		} else {
			cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("ST RAM %d (0x%04x) into RAM %d", ins.Params[0].Value, source, ins.Params[1].Value%int64(cw.Config.IdxMod)))
		}
		return true
	}

	// add. sub. Adds/Substacts the 1st and 2nd params and stores the result in the 3rd param.
	// Updates the carry.
	// 3 Params: All registers.
	ops[0x04] = mathOp(opAdd)
	ops[0x05] = mathOp(opSub)

	// and. or. xor. Bitwise AND/OR/XOR of the 1st and 2nd params and stores the result in the 3rd param.
	// Updates the carry.
	// 3 Params:
	// - 1: Can be a register, direct or indirect value.
	// - 2: Can be a register, direct or indirect value.
	// - 3: Always a register, update the content.
	ops[0x06] = mathOp(opAnd)
	ops[0x07] = mathOp(opOr)
	ops[0x08] = mathOp(opXor)

	// zjmp. Jump to the address PC + param % IDX_MOD if the carry
	// is set to 1.
	// 1 Param: Always a direct value as index (int16).
	ops[0x09] = func(cw *Corewar, p *Process) bool {
		if !p.Carry {
			return true // Advance the PC.
		}
		ins := p.CurInstruction

		// `zjmp %23` puts, if carry equals 1, PC + 23 % IDX_MOD into the PC.
		newPC := int32(p.PC) + (int32(int16(ins.Params[0].Value)) % int32(cw.Config.IdxMod))
		if newPC < 0 {
			newPC += int32(len(cw.Ram))
		}
		newPC %= int32(len(cw.Ram))
		p.PC = uint32(newPC)
		return false // Manual overrode the PC, don't advance it.
	}

	// ldi. Load index. Adds the 1st and 2nd params and loads the result into the 3rd param.
	// Updates the carry.
	// 3 Params:
	// - 1: Can be a register, direct or indirect value as index (int16).
	// - 2: Can be a register or direct value as index (int16).
	// - 3: Always a register, update the content.
	ops[0x0a] = func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		mod := int32(cw.Config.IdxMod)
		if ins.OpCode.Code == 0x0e { // lldi is the same as ldi but without modulo.
			mod = 1
		}

		var source1, source2 int16

		// Target register.
		target := ins.Params[2].Value - 1

		// Resolve the 1st and 2nd params.
		if ins.Params[0].Typ == op.TReg {
			// If register, takes the value.
			source1 = int16(p.Registers[ins.Params[0].Value-1])
		} else if ins.Params[0].Typ == op.TDir {
			// If direct, use it directly.
			source1 = int16(ins.Params[0].Value)
		} else {
			// If indirect, read int16 (2) bytes from RAM at PC + <val> % IDX_MOD.
			cw.Messages <- NewMessage(MsgPause, nil, "")
			source1 = int16(cw.Ram.GetRamValue16(p, uint32(int32(p.PC)+(int32(int16(ins.Params[0].Value))%mod))))
		}
		if ins.Params[1].Typ == op.TReg {
			source2 = int16(p.Registers[ins.Params[1].Value-1])
		} else {
			source2 = int16(ins.Params[1].Value)
		}

		// ldi 3,%4,r1 reads IND_SIZ bytes from the address PC + 3 % IDX_MOD, adds 4 to this value.
		// The sum is named S.
		// REG_SIZE bytes are read from the address PC + S % IDX_MOD and copied into r1.
		S := source1 + source2
		p.Registers[target] = cw.Ram.GetRamValue32(p, uint32(int32(p.PC)+int32(S)%mod))

		return true
	}

	// sti. Store index. Store register value at the address of 1st param + 2nd params.
	// 3 Params:
	// - 1: Always a register, take the content.
	// - 2: Can be a register, direct or indirect value as index (int16).
	// - 3: Can be a register or direct value as index (int16).
	ops[0x0b] = func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		var target1, target2 int16

		// Source register.
		source := p.Registers[ins.Params[0].Value-1]

		// Resolve the 1st and 2nd params.
		if ins.Params[1].Typ == op.TReg {
			// If register, takes the value.
			target1 = int16(p.Registers[ins.Params[1].Value-1])
		} else if ins.Params[1].Typ == op.TDir {
			// If direct, use it directly.
			target1 = int16(ins.Params[1].Value)
		} else {
			// If indirect, read int16 (2) bytes from RAM at PC + <val> % IDX_MOD.
			target1 = int16(cw.Ram.GetRamValue16(p, uint32(int32(p.PC)+(int32(int16(ins.Params[1].Value))%int32(cw.Config.IdxMod)))))
		}
		if ins.Params[2].Typ == op.TReg {
			target2 = int16(p.Registers[ins.Params[2].Value-1])
		} else {
			target2 = int16(ins.Params[2].Value)
		}

		cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("STI R%d %s", ins.Params[0].Value, ins))
		// `sti r2,%4,%5` copies the content of r2 into the address PC + (4+5) % IDX_MOD.
		S := target1 + target2
		cw.Ram.SetRamValue(p, uint32(int32(p.PC)+int32(S)%int32(cw.Config.IdxMod)), source)

		return true
	}

	// fork.
	// Fork the current process at the address PC + param % IDX_MOD.
	// 1 Param: Always a direct value as index (int16).
	ops[0x0c] = func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		mod := int64(cw.Config.IdxMod)
		if ins.OpCode.Code == 0x0f { // lfork is the same s fork but without modulo.
			mod = 1
		}
		newProcess := *p
		newProcess.CurInstruction = nil
		newProcess.PC = uint32((int64(p.PC) + (int64(int16(ins.Params[0].Value)) % mod)) % int64(len(cw.Ram)))
		newProcess.ID = cw.NextPID
		cw.NextPID++
		cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("Forking process %d to %d", p.ID, newProcess.ID))
		cw.Processes = append(cw.Processes, &newProcess)
		p.Player.ProcessCount++
		return true
	}

	// lld. Long load. Same as ld but without the IDX_MOD.
	// Updates the carry.
	// 2 Params:
	// - 1: Can be a direct value or an indirect value.
	// - 2: Always a register, update the content.
	ops[0x0d] = ops[0x02]

	// lldi. Long load index. Same as ldi but without the IDX_MOD.
	// Updates the carry.
	ops[0x0e] = ops[0x0a]

	// lfork. Long fork. Same as fork but without the IDX_MOD.
	ops[0x0f] = ops[0x0c]

	// aff. Display the value of the 1st param.
	// 1 Param: Always a register.
	ops[0x10] = func(cw *Corewar, p *Process) bool {
		ins := p.CurInstruction

		// Target register.
		r := ins.Params[0].Value - 1

		cw.Messages <- NewMessage(MsgDisplay, p, fmt.Sprintf("%c", p.Registers[r]%256))

		return true
	}

	return ops
}()

// Returns true if the PC should be updated.
func (cw *Corewar) Exec(p *Process) bool {
	ins := p.CurInstruction

	// Sanity check.
	for _, elem := range ins.Params {
		switch elem.Typ {
		case op.TReg:
			// TODO: Document this in the readme.
			if elem.Value <= 0 || int(elem.Value) > len(p.Registers)-1 {
				// Invalid instruction.
				return true
			}
		case op.TDir, op.TInd:
		default:
			// NOTE: op.TLab is not used.
			// Invalid instruction.
			return true
		}
	}

	cw.Messages <- NewMessage(MsgDebug, p, fmt.Sprintf("2Executing %s %v", ins, ins.Params))
	f, ok := ops[int(ins.OpCode.Code)]
	if !ok {
		return true
	}
	return f(cw, p)
}

// ProcessTurn executes the current process' instruction.
func (cw *Corewar) ProcessTurn(p *Process) error {
	// If the player is waiting for it's instruction to be executed,
	// nothing to do.
	// WaitCycle gets decremented by NextCycle().
	if p.WaitCycles > 0 {
		return nil
	}

	// If we had an instruction buffered, execute it.
	if p.CurInstruction != nil {
		if cw.Exec(p) {
			p.PC += uint32(p.CurInstruction.Size)
			p.PC %= uint32(len(cw.Ram))
		}
	}

	// Decode the next instruction.
	// NOTE: The longest instruction is 20 bytes. 4 bytes for instructions, 4 bytes each params, 4 params max.
	ins, _, err := parser.DecodeNextInstruction(cw.Ram.Bytes(p.PC, 4+4*op.MaxArgsNumber))
	if err != nil {
		// If the instruction is not valid, we consider it as a no-op.
		p.PC++
		p.PC %= uint32(len(cw.Ram))
		p.WaitCycles = 1
		return nil
	}
	p.CurInstruction = ins
	p.WaitCycles = int(ins.OpCode.Cycles)

	return nil
}

func (cw *Corewar) Round() error {
	// Check if we need to update the cycles to die.
	if cw.LiveCalls >= cw.Config.NumLives {
		cw.LiveCalls = 0
		cw.Config.CyclesToDie -= cw.Config.CycleDelta
	}
	// Check for death.
	if cw.CurCyclesToDie == 0 {
		// CurCyclesToDie is expired, check for players that are dead.
		for _, p := range cw.Players {
			if p.Dead {
				continue
			}
			if p.CurrentLives == 0 {
				p.Dead = true
				// NOTE: We don't reset the process count to display it.
				// Delete the processes themselves though.
				cw.Processes = slices.DeleteFunc(cw.Processes, func(process *Process) bool {
					return process.Player.Number == p.Number
				})
				cw.Messages <- NewMessage(MsgDead, &Process{ID: p.Number, Player: p}, fmt.Sprintf("Player %d (%s) died", p.Number, p.Name))
				continue
			}
			p.CurrentLives = 0
		}

		if len(cw.Processes) <= 1 {
			return io.EOF
		}

		// Reset CurCyclesToDie.
		cw.CurCyclesToDie = cw.Config.CyclesToDie
		if cw.CurCyclesToDie <= 0 {
			var tie []string
			for _, elem := range cw.Players {
				if !elem.Dead {
					tie = append(tie, fmt.Sprintf("%d (%s)", elem.Number, elem.Name))
				}
			}
			// TODO: Keep track of when was the last live called for each player as this scenario may not be a tie.
			cw.Messages <- NewMessage(MsgGameOver, nil, fmt.Sprintf("Game over, tie %d players: %s", len(tie), strings.Join(tie, ",")))
			return io.EOF
		}
	}

	// Check for game over.
	alive := 0
	for _, elem := range cw.Players {
		if !elem.Dead {
			alive++
		}
	}
	if alive <= 1 {
		cw.Messages <- NewMessage(MsgGameOver, nil, fmt.Sprintf("Game over, %d players alive", alive))
		return io.EOF
	}

	for _, p := range cw.Processes {
		if err := cw.ProcessTurn(p); err != nil {
			return fmt.Errorf("failed to execute process %d (player %d) turn: %w", p.ID, p.Player.Number, err)
		}
	}
	cw.NextCycle()

	buf, _ := json.Marshal(cw.Ram)
	cw.Messages <- NewMessage(MsgDump, nil, string(buf))

	return nil
}

func NewCorewar(cfg Config) *Corewar {
	headerlen, _, _ := op.HeaderStructSize()

	// Make sure the given player list is sorted by number.
	slices.SortFunc(cfg.Players, func(a, b PlayerConfig) int { return a.Number - b.Number })

	players := make([]*Player, 0, len(cfg.Players))
	processes := make([]*Process, 0, len(cfg.Players))
	ram := make(Ram, cfg.MemSize)
	nextPID := 1
	for i, pCfg := range cfg.Players {
		p, err := (&parser.Program{}).Decode(pCfg.Data, false)
		if err != nil {
			log.Fatalf("failed to decode player %d: %s", i, err)
		}

		player := &Player{
			Name:         p.GetDirective(op.NameCmdString),
			Number:       pCfg.Number,
			ProcessCount: 1,
		}
		players = append(players, player)
		process := &Process{
			ID:     nextPID,
			Player: player,
			PC:     uint32((cfg.MemSize / len(cfg.Players)) * i),
		}
		nextPID++
		process.Registers[0] = uint32(player.Number) // R1 gets intialized to the player number.
		processes = append(processes, process)
		for i, elem := range pCfg.Data[headerlen:] {
			ram[process.PC+uint32(i)%uint32(len(ram))] = RamEntry{
				Value:   elem,
				Process: process,
			}
		}
	}

	cw := &Corewar{
		Config: cfg,

		Ram:       ram,
		Players:   players,
		Processes: processes,
		NextPID:   nextPID,

		Cycle:          0,
		CurCyclesToDie: cfg.CyclesToDie,

		Messages: make(chan Message, 10), // Arbitrary size.
	}

	buf, _ := json.Marshal(cw.Ram)

	cw.Messages <- NewMessage(MsgDump, nil, string(buf))

	return cw
}
