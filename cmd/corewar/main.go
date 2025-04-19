package main

import (
	"bytes"
	"fmt"
	"log"
	"slices"
	"time"

	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/op"
)

func dump(vm []byte, pc uint32) {
	zz := make([]byte, 32)
	for i := 0; i < len(vm); {
		b := vm[i]
		if i%32 == 0 {
			if bytes.Equal(vm[i:i+32], zz) {
				fmt.Printf("\n*")
				for ; i < len(vm) && bytes.Equal(vm[i:i+32], zz); i += 32 {
				}
				continue
			}
			fmt.Printf("\n0x%04X:", i)
		}
		if i == int(pc) {
			fmt.Printf("\033[7m")
		}
		fmt.Printf(" %02x", b)
		if i == int(pc) {
			fmt.Printf("\033[27m")
		}
		i++
	}
	fmt.Printf("\n")
}

type Player struct {
	Name      string
	Number    int
	Registers [op.RegisterCount]uint32
	PC        uint32
	Carry     bool
	Dead      bool

	curInstruction *parser.Instruction
	waitCycles     int
	lastAlive      int // Cycle since last seen alive.
}

type Corewar struct {
	ram []byte

	players   []*Player
	curPlayer int
}

// TODO: Add proper support for circular memory, modulor is not enough,
//
//	if we read at op.MemSize-1 4 bytes, it should read 3 bytes at the start.
func (cw *Corewar) GetRamValue32(addr uint32) uint32 {
	return op.Endian.Uint32(cw.ram[addr%uint32(len(cw.ram)):])
}

func (cw *Corewar) GetRamValueIndex(pc uint32, idx int16, mod int64) uint16 {
	return op.Endian.Uint16(cw.ram[(int64(pc)+(int64(idx)%mod))%int64(len(cw.ram)):])
}

func (cw *Corewar) SetRamValue(addr uint32, value uint32) {
	op.Endian.PutUint32(cw.ram[addr%uint32(len(cw.ram)):], value)
}

// Returns true if the PC should be updated.
func (cw *Corewar) Exec(p *Player) bool {
	ins := p.curInstruction

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
		i := slices.IndexFunc(cw.players, func(p *Player) bool { return p.Number == int(params[0]) })
		if i == -1 || i >= len(cw.players) {
			break
		}
		targetPlayer := cw.players[i]
		fmt.Printf("Player %d (%s) is alive\n", targetPlayer.Number, targetPlayer.Name)
	case 0x03: // st.
		cw.SetRamValue(params[1], params[0])
	case 0x02, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0a, 0x0d, 0x0e:
		r := params[len(params)-1] - 1 // The register is the last parameter, -1 as it starts at 1.
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
		newPC := uint32((int64(p.PC) + (int64(int16(params[0])) % op.IdxMod)) % int64(len(cw.ram)))
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
		newPlayer.PC = uint32((int64(p.PC) + (int64(int16(params[0])) % mod)) % int64(len(cw.ram)))
		cw.players = append(cw.players, &newPlayer)
	case 0x10: // aff.
		fmt.Printf("%c\n", params[0]%256)
	}

	return true
}

func (cw *Corewar) playerTurn() error {
	p := cw.players[cw.curPlayer]

	// If the player is waiting for it's instruction to be executed,
	// decrement the wait cycles and return.
	if p.waitCycles > 0 {
		p.waitCycles--
		return nil
	}

	// If we had an instruction buffered, execute it.
	if p.curInstruction != nil {
		if cw.Exec(p) {
			p.PC += uint32(p.curInstruction.Size)
		}
	}

	// Decode the next instruction.
	ins, _, err := parser.DecodeNextInstruction(cw.ram[p.PC:])
	if err != nil {
		// If the instruction is not valid, we consider it as a no-op.
		p.PC++
		return nil
	}
	p.curInstruction = ins
	p.waitCycles = int(ins.OpCode.Cycles)

	return nil
}

func NewCorewar(memSize int) *Corewar {
	headerlen, _, _ := op.ChampionHeader{}.StructSize()

	playersData := [][]byte{
		// dataDeathIvan,
		// dataDeathMy,
		// dataZorkMy,
		// dataZorkMy,
	}
	players := make([]*Player, 0, len(playersData))
	ram := make([]byte, op.MemSize)
	for i, data := range playersData {
		p, err := (&parser.Program{}).Decode(data, false)
		if err != nil {
			log.Fatalf("failed to decode player %d: %s", i, err)
		}

		player := &Player{
			Name:   p.GetDirective("name"),
			Number: i + 1,
			PC:     uint32((memSize / len(playersData)) * i),
		}
		player.Registers[0] = uint32(player.Number) // R1 gets intialized to the player number.
		players = append(players, player)

		copy(ram[player.PC:], playersData[i][headerlen:])
	}

	return &Corewar{
		ram:     ram,
		players: players,
	}
}

func test() error {
	cw := NewCorewar(op.MemSize)
	if len(cw.players) == 0 {
		return fmt.Errorf("no players found")
	}
	dump(cw.ram, 0)

	cw.curPlayer = 0
	for range 30 {
		fmt.Printf("\033[2J\033[H%d\n", cw.players[cw.curPlayer].PC)
		if err := cw.playerTurn(); err != nil {
			return fmt.Errorf("failed to execute instruction: %w", err)
		}
		dump(cw.ram, cw.players[cw.curPlayer].PC)
		cw.curPlayer++
		cw.curPlayer %= len(cw.players)
		time.Sleep(100e6)
	}

	return nil
}

func main() {
	if err := test(); err != nil {
		println("Fail:", err.Error())
		return
	}
}
