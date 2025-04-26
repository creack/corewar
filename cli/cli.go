// Package cli provides the functions to parse the non-standard CLI flags.
package cli

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"go.creack.net/corewar/asm"
	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/disasm"
	"go.creack.net/corewar/op"
	"go.creack.net/corewar/vm"
)

const MaxPlayers = 4

type Player struct {
	PathName  string
	ShortName string
	Number    int
	Data      []byte

	Prog *parser.Program
}

func parse() ([]*Player, error) {
	// Define a variable to hold the -n value temporarily
	var number int

	var players []*Player

	// Process arguments manually
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "-n" && i+1 < len(args) {
			num, err := strconv.Atoi(args[i+1])
			if err != nil {
				return nil, fmt.Errorf("invalid number for -n flag: %q", args[i+1])
			}
			number = num
			i++ // Skip the value of -n
			continue
		} else if strings.HasPrefix(arg, "-n") {
			arg = strings.TrimPrefix(arg, "-n")
			num, err := strconv.Atoi(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid number for -n flag: %q", arg)
			}
			number = num
			continue
		}

		// If it's not a flag, it's a player name
		if arg[0] != '-' {
			players = append(players, &Player{PathName: arg, Number: number})
			number = 0 // Reset for the next player
		}
	}
	if len(players) == 0 {
		return nil, fmt.Errorf("no players provided")
	}

	// Make sure we don't have a duplicate number.
	inputNumbers := map[int]string{}
	// Create a list with the available player numbers.
	numbers := make([]int, MaxPlayers)
	for i := range numbers { // Populate the list.
		numbers[i] = i + 1
	}
	// Go over the parsed players, and remove from the available list the numbers we already have.
	for _, p := range players {
		if !strings.HasSuffix(p.PathName, ".s") && !strings.HasSuffix(p.PathName, ".cor") {
			return nil, fmt.Errorf("invalid file extension for %q, must be .s or .cor", p.PathName)
		}
		if p.Number == 0 {
			continue
		}
		if p.Number < 1 || p.Number > MaxPlayers {
			return nil, fmt.Errorf("invalid player number: %d for %q, must be between 1 and %d", p.Number, p.PathName, MaxPlayers)
		}
		if n, ok := inputNumbers[p.Number]; ok {
			return nil, fmt.Errorf("duplicate player number: %d, used for %q and %q", p.Number, p.PathName, n)
		}
		inputNumbers[p.Number] = p.PathName
		numbers = slices.DeleteFunc(numbers, func(elem int) bool { return elem == p.Number })
	}

	// Allocate next available number to players that don't have one.
	for _, n := range numbers {
		for _, p := range players {
			if p.Number == 0 {
				p.Number = n
				break
			}
		}
	}

	return players, nil
}

func loadPlayers(players []*Player) error {
	for _, p := range players {
		tmp := strings.Split(p.PathName, "/")
		p.ShortName = tmp[len(tmp)-1]
		p.ShortName = strings.TrimSuffix(p.ShortName, ".s")
		p.ShortName = strings.TrimSuffix(p.ShortName, ".cor")

		data, err := os.ReadFile(p.PathName)
		if err != nil {
			return fmt.Errorf("failed to read file %q: %w", p.PathName, err)
		}
		if strings.HasSuffix(p.PathName, ".s") {
			buf, pr, err := asm.Compile(p.PathName, string(data), false)
			if err != nil {
				return fmt.Errorf("failed to compile %q: %w", p.PathName, err)
			}
			p.Prog = pr
			data = buf
		}
		p.Data = data

		prog, err := disasm.Disam(p.ShortName, data, false)
		if err != nil {
			return fmt.Errorf("failed to disassemble %q: %w", p.PathName, err)
		}
		p.Prog = prog
	}
	return nil
}

func ParseConfig() (vm.Config, []*Player, error) {
	players, err := parse()
	if err != nil {
		return vm.Config{}, nil, fmt.Errorf("parse: %w", err)
	}
	if err := loadPlayers(players); err != nil {
		return vm.Config{}, nil, fmt.Errorf("load players: %w", err)
	}

	cfg := vm.Config{
		MemSize:     op.MemSize,
		IdxMod:      op.IdxMod,
		CyclesToDie: op.CyclesToDie,
		CycleDelta:  op.CycleDelta,
		NumLives:    op.NumLives,
		Players:     make([]vm.PlayerConfig, 0, len(players)),
	}
	for _, p := range players {
		cfg.Players = append(cfg.Players, vm.PlayerConfig{
			Number: p.Number,
			Data:   p.Data,
		})
	}
	return cfg, players, nil
}
