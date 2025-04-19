// Package main is the entry point of the program.
package vm

import (
	_ "embed"
	"fmt"
	"log"
	"slices"

	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/op"
)

type Player struct {
	Name      string
	Number    int
	Registers [op.RegisterCount]uint32
	PC        uint32
	Carry     bool
	Dead      bool

	CurInstruction *parser.Instruction
	WaitCycles     int
	LastAlive      int // Cycle since last seen alive.
}

type Corewar struct {
	Ram []byte

	Players   []*Player
	CurPlayer int
	Cycle     int
}

// NextCycle advances the cycle counter until a player is ready to go.
// Useful when everyone is waiting for a long instruction like fork.
func (cw *Corewar) NextCycle() {
	cycles := -1

	for _, p := range cw.Players {
		if cycles == -1 || p.WaitCycles < cycles {
			cycles = p.WaitCycles
		}
	}
	if cycles == -1 {
		cycles = 1
	}

	cw.Cycle += cycles
	for _, p := range cw.Players {
		p.WaitCycles -= cycles
	}
}

// TODO: Add proper support for circular memory, modulor is not enough,
//
//	if we read at op.MemSize-1 4 bytes, it should read 3 bytes at the start.
func (cw *Corewar) GetRamValue32(addr uint32) uint32 {
	return op.Endian.Uint32(cw.Ram[addr%uint32(len(cw.Ram)):])
}

func (cw *Corewar) GetRamValueIndex(pc uint32, idx int16, mod int64) uint16 {
	return op.Endian.Uint16(cw.Ram[(int64(pc)+(int64(idx)%mod))%int64(len(cw.Ram)):])
}

func (cw *Corewar) SetRamValue(addr uint32, value uint32) {
	op.Endian.PutUint32(cw.Ram[addr%uint32(len(cw.Ram)):], value)
}

// Returns true if the PC should be updated.
func (cw *Corewar) Exec(p *Player) bool {
	ins := p.CurInstruction

	// NOTE: The parameters have already been validated.
	params := make([]uint32, 0, len(ins.Params))
	for _, elem := range ins.Params {
		switch {
		case elem.Typ == op.TReg:
			// TODO: Document this in the readme.
			if elem.Value <= 0 || int(elem.Value) > len(p.Registers)-1 {
				// Invalid instruction.
				return true
			}
			params = append(params, p.Registers[elem.Value-1])

		case elem.Typ == op.TDir:
			params = append(params, uint32(elem.Value))

		case elem.Typ == op.TInd:
			mod := int64(op.IdxMod)
			if c := ins.OpCode.Code; c == 0x0d || c == 0x0e || c == 0x0f {
				// lld, lldi, lfork don't apply modulo.
				mod = 1
			}
			params = append(params, uint32(cw.GetRamValueIndex(p.PC, int16(elem.Value), mod)))
		default:
			// NOTE: op.TLab is not used.
			log.Fatal("unknwown parameter type/mode", ins)
		}
	}

	switch ins.OpCode.Code {
	case 0x00: // noop.
	case 0x01: // live.
		i := slices.IndexFunc(cw.Players, func(p *Player) bool { return p.Number == int(params[0]) })
		if i == -1 || i >= len(cw.Players) || cw.Players[i].Dead {
			break
		}
		targetPlayer := cw.Players[i]
		_ = targetPlayer
		//fmt.Printf("Player %d (%s) is alive\n", targetPlayer.Number, targetPlayer.Name)
	case 0x03: // st.
		cw.SetRamValue(params[1], params[0])
	case 0x02, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0a, 0x0d, 0x0e:
		r := params[len(params)-1] // The register is the last parameter.
		switch ins.OpCode.Code {
		case 0x02, 0x0d: // ld, lld.
			p.Registers[r] = params[0]
		case 0x04: // add.
			p.Registers[r] = params[0] + params[1]
		case 0x05: // sub.
			p.Registers[r] = params[0] - params[1]
		case 0x06: // and.
			p.Registers[r] = params[0] & params[1]
		case 0x07: // or.
			p.Registers[r] = params[0] | params[1]
		case 0x08: // xor.
			p.Registers[r] = params[0] ^ params[1]
		case 0x0a: // ldi.
			idx := int16(params[1]) + int16(params[2])
			p.Registers[r] = cw.GetRamValue32(uint32(idx % op.IdxMod))
		case 0x0e: // lldi.
			idx := int16(params[1]) + int16(params[2])
			p.Registers[r] = cw.GetRamValue32(uint32(idx))
		}
		p.Carry = p.Registers[r] == 0
	case 0x09: // zjmp.
		if !p.Carry {
			break
		}
		newPC := uint32((int64(p.PC) + (int64(int16(params[0])) % op.IdxMod)) % int64(len(cw.Ram)))
		//time.Sleep(20e9)
		p.PC = newPC
		return false
	case 0x0b: // sti.
		idx := int16(params[1]) + int16(params[2])
		cw.SetRamValue(uint32(idx), params[0])
	case 0x0c, 0x0f: // fork, lfork.
		mod := int64(op.IdxMod)
		if ins.OpCode.Code == 0x0f { // lfork is the same s fork but without modulo.
			mod = 1
		}
		newPlayer := *p
		newPlayer.CurInstruction = nil
		newPlayer.PC = uint32((int64(p.PC) + (int64(int16(params[0])) % mod)) % int64(len(cw.Ram)))
		cw.Players = append(cw.Players, &newPlayer)
	case 0x10: // aff.
		fmt.Printf("%c\n", params[0]%256)
	}

	return true
}

// PlayerTurn executes the current player's instruction.
func (cw *Corewar) PlayerTurn() error {
	defer func() {
		cw.CurPlayer++
		cw.CurPlayer %= len(cw.Players)
	}()
	p := cw.Players[cw.CurPlayer]

	// If the player is waiting for it's instruction to be executed,
	// nothnig to do.
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
	ins, _, err := parser.DecodeNextInstruction(cw.Ram[p.PC:])
	if err != nil {
		// If the instruction is not valid, we consider it as a no-op.
		p.PC++
		p.PC %= uint32(len(cw.Ram))
		return nil
	}
	p.CurInstruction = ins
	p.WaitCycles = int(ins.OpCode.Cycles)

	return nil
}

func (cw *Corewar) Round() error {
	players := cw.Players
	for _, p := range players {
		if err := cw.PlayerTurn(); err != nil {
			// TODO: Consider adding a process id.
			return fmt.Errorf("failed to execute player %d turn: %w", p.Number, err)
		}
	}
	return nil
}

func NewCorewar(memSize int, playersData [][]byte) *Corewar {
	headerlen, _, _ := op.HeaderStructSize()

	players := make([]*Player, 0, len(playersData))
	ram := make([]byte, op.MemSize)
	for i, data := range playersData {
		p, err := (&parser.Program{}).Decode(data, false)
		if err != nil {
			log.Fatalf("failed to decode player %d: %s", i, err)
		}

		player := &Player{
			Name:   p.GetDirective(op.NameCmdString),
			Number: i + 1,
			PC:     uint32((memSize / len(playersData)) * i),
		}
		player.Registers[0] = uint32(player.Number) // R1 gets intialized to the player number.
		players = append(players, player)

		copy(ram[player.PC:], playersData[i][headerlen:])
	}

	return &Corewar{
		Ram:     ram,
		Players: players,
	}
}
