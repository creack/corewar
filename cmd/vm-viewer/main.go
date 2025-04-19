package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"go.creack.net/corewar/asm"
	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/disasm"
	"go.creack.net/corewar/op"
	"go.creack.net/corewar/vm"
)

func cStrLen(buf []byte) int {
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return i
}

// bannedColors that are not legible.
var bannedColors = []int{
	0,
	16,
	17,
	18,
	19,
	20,
	21,
	52,
	53,
	54,
	55,
	232,
	233,
	234,
	235,
	236,
	237,
	238,
	239,
}

var curColor = 0

func nextColor() int {
	curColor++
	curColor %= 256
	for slices.Contains(bannedColors, curColor) {
		curColor++
		curColor %= 256
	}
	return curColor
}

func colorCodeModif(color int, mods ...int) string {
	modsStr := make([]string, 0, len(mods))
	for _, elem := range mods {
		modsStr = append(modsStr, fmt.Sprintf("%d", elem))
	}
	ansiMod := strings.Join(modsStr, ";")
	if ansiMod != "" {
		ansiMod += ";"
	}
	return fmt.Sprintf("\033[%s38;5;%dm", ansiMod, color)
}

func colorCodef(color int) string {
	return colorCodeModif(color)
}

func nextColorCode() string {
	return colorCodef(nextColor())
}

func dump(vm []byte, players []*vm.Player) string {
	out := &strings.Builder{}

	const width = 64

	i := 0
	for i < len(vm) {

		b := vm[i]
		if i%width == 0 {
			if i != 0 {
				fmt.Fprintf(out, "\n")
			}
			fmt.Fprintf(out, "0x%04x", i)
		}
		if i%(width/4) == 0 {
			fmt.Fprintf(out, " ")
		}
		selectedCode := ""
		for _, p := range players {
			if i == int(p.PC) {
				selectedCode = "\033[7m"
			}
		}
		fmt.Fprintf(out, " %s%02x\033[0m", selectedCode, b)

		i++
	}
	for i%width != 0 {
		i++
	}
	fmt.Fprintf(out, "\n0x%04x", i)

	return out.String()
}

func NewGame(ctx context.Context, cw *vm.Corewar) *Game {
	app := tview.NewApplication().EnableMouse(true)

	newTextView := func(text string) *tview.TextView {
		return tview.NewTextView().
			SetDynamicColors(true).
			SetText(text)
	}

	ramView := newTextView("RAM")
	_ = ramView
	// tview.ANSIWriter(leftContent).Write([]byte(intput))

	processListView := tview.NewTable().SetBorders(false)
	processListView.SetTitle("Processes").SetBorder(true)

	settingsView := newTextView("Settings")
	settingsView.SetTitle("Settings").SetBorder(true)

	playersListView := newTextView("Players")
	playersListView.SetBorder(true)
	playersListView.SetTitle("Players")

	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)
	rightPane.
		AddItem(settingsView, 0, 1, false).
		AddItem(playersListView, 0, 2, false).
		AddItem(processListView, 0, 4, false)

	ramPane := tview.NewFlex()
	ramPane.SetBorder(true)
	ramPane.SetTitle("RAM")
	ramPane.AddItem(ramView, 0, 1, false)

	flex := tview.NewFlex().
		AddItem(ramPane, 0, 3, true).
		AddItem(rightPane, 0, 1, false)
	_ = flex

	ctx, cancel := context.WithCancel(ctx)

	return &Game{
		app: app,

		root:            flex,
		ramView:         ramView,
		processListView: processListView,
		settingsView:    settingsView,

		cw:     cw,
		ctx:    ctx,
		cancel: cancel,

		paused: true,
	}
}

func parseCLI() ([]*CLIPlayer, error) {
	const MaxPlayers = 4
	// Define a variable to hold the -n value temporarily
	var number int

	var players []*CLIPlayer

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
		}

		// If it's not a flag, it's a player name
		if arg[0] != '-' {
			players = append(players, &CLIPlayer{PathName: arg, Number: number})
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
		numbers = slices.DeleteFunc(numbers, func(elem int) bool {
			return elem == p.Number
		})
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

type CLIPlayer struct {
	PathName  string
	ShortName string
	Number    int
	Data      []byte
	Prog      *parser.Program
}

func loadPlayers(players []*CLIPlayer) error {
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
			continue
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

type Game struct {
	app *tview.Application

	root *tview.Flex

	ramView         *tview.TextView
	processListView *tview.Table
	settingsView    tview.Primitive

	cw *vm.Corewar

	paused   bool
	pausedMu sync.Mutex

	nextStep   bool
	nextStepMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

var Termination = errors.New("termination")

func (g *Game) Stop() {
	g.app.Stop()
	g.cancel()
}

func (g *Game) Init() {
	f := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyEscape:
			g.Stop()
			return nil
		}
		switch event.Rune() {
		case 'n':
			g.nextStepMu.Lock()
			g.nextStep = true
			g.nextStepMu.Unlock()

		case ' ':
			g.pausedMu.Lock()
			g.paused = !g.paused
			g.pausedMu.Unlock()
		case 'q':
			g.Stop()
			return nil
		}
		return event
	}
	g.root.SetInputCapture(f)
}

func (g *Game) Update() error {
	isPaused := func() bool {
		g.pausedMu.Lock()
		defer g.pausedMu.Unlock()
		return g.paused
	}
	forceNextStep := func() bool {
		g.nextStepMu.Lock()
		defer g.nextStepMu.Unlock()
		if g.nextStep {
			g.nextStep = false
			return true
		}
		return false
	}
	if !forceNextStep() && isPaused() {
		return nil
	}
	g.cw.NextCycle()

	if err := g.cw.Round(); err != nil {
		return fmt.Errorf("failed to execute instruction: %w", err)
	}

	return nil
}

func (g *Game) Draw() {
	g.ramView.Clear()
	w := tview.ANSIWriter(g.ramView)
	io.Copy(w, strings.NewReader(dump(g.cw.Ram, g.cw.Players)))

	sv := g.settingsView.(*tview.TextView)
	sv.Clear()
	g.nextStepMu.Lock()
	sv.SetText(fmt.Sprintf("next: %t\n", g.nextStep))
	g.nextStepMu.Unlock()

	for i, elem := range []string{
		"ppid",
		"pc",
		"op",
		"wait",
		"registers",
		"carry",
	} {
		cell := tview.NewTableCell(elem).
			SetAttributes(tcell.AttrBold).
			SetAlign(tview.AlignCenter)

		g.processListView.SetCell(0, i, cell).SetFixed(1, i)
	}

	dumpRegisters := func(regs []uint32) string {
		parts := make([]string, 0, len(regs))
		for _, elem := range regs {
			val := "."
			if elem != 0 {
				val = "x"
			}
			parts = append(parts, val)
		}
		return strings.Join(parts, "")
	}
	g.processListView.SetTitle(fmt.Sprintf("Processes (%d)", len(g.cw.Players)))
	for i, elem := range g.cw.Players {
		curInsName := ""
		if elem.CurInstruction != nil {
			curInsName = elem.CurInstruction.OpCode.Name
		}
		for j, content := range []any{
			elem.Number,
			fmt.Sprintf("%04x", elem.PC),
			curInsName,
			elem.WaitCycles,
			dumpRegisters(elem.Registers[:]),
			elem.Carry,
		} {
			g.processListView.SetCell(i+1, j, tview.NewTableCell(fmt.Sprint(content)).SetAlign(tview.AlignRight))
		}
	}
}

func main() {
	players, err := parseCLI()
	if err != nil {
		log.Fatalf("failed to parse CLI: %s.", err)
	}
	if err := loadPlayers(players); err != nil {
		log.Fatalf("Failed to load players: %s.", err)
	}

	playersData := make([][]byte, len(players))
	for i, p := range players {
		playersData[i] = p.Data
	}
	cw := vm.NewCorewar(op.MemSize, playersData)

	g := NewGame(context.Background(), cw)

	g.Init()
	go func() {
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()
	loop:
		func() {
			defer func() {
				if e := recover(); e != nil {
					g.app.Stop()
					log.Printf("Recovered from panic: %v", e)
					debug.PrintStack()
				}
			}()
			if err := g.Update(); err != nil {
				if errors.Is(err, Termination) {
					g.Stop()
					return
				}
				log.Printf("failed to update: %s", err)
			}
		}()

		g.app.QueueUpdateDraw(func() {
			g.Draw()
		})

		select {
		case <-ticker.C:
		case <-g.ctx.Done():
			g.Stop()
			return
		}
		goto loop
	}()

	if err := g.app.SetRoot(g.root, true).SetFocus(g.root).Run(); err != nil {
		panic(err)
	}
	log.Printf("Done")
}
