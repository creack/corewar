package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"go.creack.net/corewar/cli"
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

var colors = []tcell.Color{
	// tcell.ColorBlue,
	// tcell.ColorLightGreen,
	// tcell.ColorRed,
	// tcell.ColorPurple,
}

func nextColor() tcell.Color {
	curColor++
	curColor %= len(colors)
	for slices.Contains(bannedColors, curColor) {
		curColor++
		curColor %= len(colors)
	}
	return colors[curColor]
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
	return colorCodef(int(nextColor()))
}

func dump(vm []byte, processes []*vm.Process) string {
	out := &strings.Builder{}

	const width = 64

	i := 0
	for i < len(vm) {
		b := vm[i]
		if i%width == 0 {
			if i != 0 {
				fmt.Fprintf(out, "\n")
			}
			// fmt.Fprintf(out, "0x%04x", i)
		}
		// if i%(width/4) == 0 {
		// 	fmt.Fprintf(out, " ")
		// }
		selectedCode := ""
		for _, p := range processes {
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
	// fmt.Fprintf(out, "\n0x%04x", i)

	return out.String()
}

func dumpChampion(data []byte) string {
	out := &strings.Builder{}
	const width = 16
	zz := make([]byte, width)
	for i := 0; i < len(data); {
		b := data[i]
		if i%width == 0 {
			if i+width <= len(data) && bytes.Equal(data[i:i+width], zz) {
				fmt.Fprintf(out, "\n*")
				for ; i < len(data) && bytes.Equal(data[i:i+width], zz); i += width {
				}
				continue
			}
			fmt.Fprintf(out, "\n0x%04X:", i)
		}
		if i%(width/2) == 0 {
			fmt.Fprintf(out, " ")
		}
		fmt.Fprintf(out, " %02x", b)
		i++
	}
	fmt.Fprintf(out, "\n")
	return out.String()
}

func NewGame(ctx context.Context, cw *vm.Corewar) *Game {
	app := tview.NewApplication().EnableMouse(true)

	newTextView := func(text string) *tview.TextView {
		return tview.NewTextView().
			SetDynamicColors(true).
			SetText(text)
	}

	//ramView := newTextView("RAM")
	// tview.ANSIWriter(leftContent).Write([]byte(intput))
	ramView := tview.NewTable().SetBorders(false)

	logsView := newTextView("")
	logsView.SetTitle("Logs").SetBorder(true)
	logsView.ScrollToEnd()

	processListView := tview.NewTable().SetBorders(false)
	processListView.SetTitle("Processes").SetBorder(true)

	stateView := newTextView("Settings")
	stateView.SetTitle("Settings").SetBorder(true)

	playersListView := tview.NewList()
	playersListView.SetBorder(true)
	playersListView.SetTitle("Players")
	playersListView.SetSelectedFocusOnly(true)

	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)
	rightPane.
		AddItem(stateView, 0, 2, false).
		AddItem(playersListView, 0, 2, false).
		AddItem(logsView, 0, 3, false).
		AddItem(processListView, 0, 4, false)

	ramPane := tview.NewFlex()
	ramPane.SetBorder(true)
	ramPane.SetTitle("RAM")
	ramPane.AddItem(ramView, 0, 1, false)

	flex := tview.NewFlex().
		AddItem(ramPane, 0, 3, true).
		AddItem(rightPane, 0, 1, false)
	_ = flex

	pages := tview.NewPages()
	pages.AddPage("main", flex, true, true)

	for _, p := range cw.Players {
		playersListView.AddItem("", "", 0, func() {
			pages.ShowPage(fmt.Sprintf("disasm-player-%d", p.Number))
		})
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Game{
		app: app,

		root: pages,

		mainPage:        flex,
		ramView:         ramView,
		processListView: processListView,
		stateView:       stateView,
		playerListView:  playersListView,
		logsView:        logsView,

		cw:     cw,
		ctx:    ctx,
		cancel: cancel,

		paused: true,
	}
}

type Game struct {
	app *tview.Application

	root *tview.Pages

	mainPage *tview.Flex

	ramView         tview.Primitive
	processListView *tview.Table
	stateView       tview.Primitive
	playerListView  tview.Primitive
	logsView        *tview.TextView

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
		curPage, _ := g.root.GetFrontPage()
		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyEscape:
			if curPage != "main" {
				g.root.SwitchToPage("main")
				return nil
			}
			g.Stop()
			return nil
		case tcell.KeyEnter:
			if curPage != "main" {
				g.root.SwitchToPage("main")
				return nil
			}
			return event
		}
		switch event.Rune() {
		case 'n':
			g.nextStepMu.Lock()
			g.nextStep = true
			g.nextStepMu.Unlock()
			return nil
		case ' ':
			if curPage == "main" {
				g.pausedMu.Lock()
				g.paused = !g.paused
				g.pausedMu.Unlock()
			} else {
				g.root.SwitchToPage("main")
			}
			return nil
		case 'q':
			if curPage != "main" {
				g.root.SwitchToPage("main")
				return nil
			}
			g.Stop()
			return nil
		}
		return event
	}
	g.root.SetInputCapture(f)
	go func() {
	loop:
		select {
		case msg := <-g.cw.Messages:
			g.app.QueueUpdateDraw(func() {
				if msg.Type == vm.MsgClear {
					g.logsView.Clear()
					return
				}
				if msg.Type == vm.MsgPause {
					g.pausedMu.Lock()
					g.paused = true
					g.pausedMu.Unlock()
					return
				}
				// NOTE: Seems like there is a bug with tview, we can't reset the color to default
				// with [:] or [:::], so we use tcell default.
				colorCode := "[" + tcell.ColorDefault.String() + ":::]"
				if msg.Process != nil {
					colorCode = "[" + colors[msg.Process.ID%len(colors)].String() + ":::]"
				}
				if msg.Process != nil {
					fmt.Fprintf(g.logsView, "%s[%d] %s[:::]\n", colorCode, msg.Process.ID, strings.TrimSuffix(msg.Message, "\n"))
				} else {
					fmt.Fprintf(g.logsView, "%s%s[:::]\n", colorCode, strings.TrimSuffix(msg.Message, "\n"))
				}
			})
			g.app.Draw()
		case <-g.ctx.Done():
			return
		}
		goto loop
	}()
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

	if err := g.cw.Round(); err != nil {
		return fmt.Errorf("failed to execute instruction: %w", err)
	}
	g.Draw()

	return nil
}

func (g *Game) drawProcessList() {
	g.processListView.SetTitle(fmt.Sprintf("Processes (%d)", len(g.cw.Processes)))
	g.processListView.Clear()
	for i, elem := range []string{
		"pid",
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
	for i, elem := range g.cw.Processes {
		curInsName := ""
		if elem.CurInstruction != nil {
			curInsName = elem.CurInstruction.OpCode.Name
		}
		for j, content := range []any{
			elem.ID,
			elem.Player.Number,
			fmt.Sprintf("%04x", elem.PC),
			curInsName,
			elem.WaitCycles,
			dumpRegisters(elem.Registers[:]),
			elem.Carry,
		} {
			cell := tview.NewTableCell(fmt.Sprint(content)).SetAlign(tview.AlignRight)
			cell.SetTextColor(colors[elem.ID%len(colors)])
			g.processListView.SetCell(i+1, j, cell)

		}
	}
}

func (g *Game) drawPlayerList() {
	pv := g.playerListView.(*tview.List)

	for i, p := range g.cw.Players {
		deadCode := ""
		if p.Dead {
			deadCode = "s"
		}
		pid := p.Number
		attr := "[" + colors[pid%len(colors)].String() + "::" + deadCode + ":]"
		txt := fmt.Sprintf("%s[%d] %s (%d)[:::]", attr, p.Number, p.Name, p.ProcessCount)
		pv.SetItemText(i, txt, "")
	}
}

func (g *Game) drawState() {
	sv := g.stateView.(*tview.TextView)
	sv.Clear()

	fmt.Fprintf(sv, "Cycles: %d\n", g.cw.Cycle)
	fmt.Fprintf(sv, "Current CyclesToDie: %d\n", g.cw.CurCyclesToDie)
	fmt.Fprintf(sv, "Next CyclesToCheck: %d\n", g.cw.Config.CyclesToDie)
	fmt.Fprintf(sv, "Memory Size: %d\n", g.cw.Config.MemSize)
	fmt.Fprintf(sv, "IdxMod: %d\n", g.cw.Config.IdxMod)
	fmt.Fprintf(sv, "NumLives: %d\n", g.cw.Config.NumLives)
	fmt.Fprintf(sv, "CycleDelta: %d\n", g.cw.Config.CycleDelta)
	fmt.Fprintf(sv, "Period live count: %d\n", g.cw.LiveCalls)
}

func (g *Game) drawRAM() {
	const width = 64
	ramView := g.ramView.(*tview.Table)
	ramView.SetSelectable(true, true)
	for i, elem := range g.cw.Ram {
		onClick := []func(){func() { g.cw.Messages <- vm.NewMessage(vm.MsgPause, nil, "") }}

		cell := tview.NewTableCell(fmt.Sprintf("%02x", elem.Value))
		if elem.Process != nil {
			cell.SetTextColor(colors[elem.Process.ID%len(colors)])
			if elem.AccessType == 1 {
				cell.SetAttributes(tcell.AttrBold)
			} else if elem.AccessType == 2 {
				cell.SetAttributes(tcell.AttrItalic | tcell.AttrDim)
			} else if elem.AccessType == 3 {
				cell.SetAttributes(tcell.AttrItalic | tcell.AttrDim | tcell.AttrUnderline | tcell.AttrBlink)
			}
			onClick = append(onClick, func() {
				g.cw.Messages <- vm.NewMessage(vm.MsgDebug, elem.Process, fmt.Sprintf("Yup!!! %d", elem.Process.ID))
			})
		} else if elem.Value == 0 {
			cell.SetTextColor(tcell.ColorDimGray)
			cell.SetAttributes(tcell.AttrDim)
		}
		for _, p := range g.cw.Processes {
			if !p.Player.Dead && i == int(p.PC) {
				cell.SetAttributes(tcell.AttrReverse).SetTextColor(colors[p.ID%len(colors)])
				onClick = append(onClick, func() {
					g.cw.Messages <- vm.NewMessage(vm.MsgDebug, p, fmt.Sprintf("PC player %d", p.Player.Number))
				})
			}
		}
		cell.SetClickedFunc(func() bool {
			go func() {
				for _, f := range onClick {
					f()
				}
			}()
			return true
		})
		ramView.SetCell(i/width, i%width, cell)
	}

	// ramView := g.ramView.(*tview.TextView)
	// ramView.Clear()
	// w := tview.ANSIWriter(ramView)
	// io.Copy(w, strings.NewReader(dump(g.cw.Ram, g.cw.Processes)))
}

func (g *Game) Draw() {
	g.drawRAM()
	g.drawState()
	g.drawPlayerList()
	g.drawProcessList()
}

func main() {
	for _, v := range tcell.ColorNames {
		colors = append(colors, v)
	}

	cfg, players, err := cli.ParseConfig()
	if err != nil {
		log.Fatalf("Failed to parse CLI config: %s.", err)
	}

	cw := vm.NewCorewar(cfg)
	if err := cw.Round(); err != nil {
		log.Fatalf("Failed to execute first round: %s.", err)
	}

	g := NewGame(context.Background(), cw)

	for _, p := range players {
		pl2 := tview.NewTextView().SetText(dumpChampion(p.Data))

		pl := tview.NewTextView().SetText(fmt.Sprintf("Player: %d (%s)\n\n", p.Number, p.Prog.GetDirective(op.NameCmdString)))
		buf := &strings.Builder{}
		for _, elem := range p.Prog.Nodes {
			fmt.Fprintf(buf, "%s\n", elem.PrettyPrint(p.Prog.Nodes))
		}
		pl.SetText(buf.String())

		flex := tview.NewFlex().AddItem(pl, 0, 1, false).
			AddItem(pl2, 0, 1, false)
		g.root.AddPage(fmt.Sprintf("disasm-player-%d", p.Number), flex, true, false)
	}

	g.Init()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
	loop:

		defer func() {
			if e := recover(); e != nil {
				g.app.Stop()
				log.Printf("Recovered from panic: %v", e)
				debug.PrintStack()
			}
		}()
		end := false
		if err := g.Update(); err != nil {
			if errors.Is(err, io.EOF) {
				end = true
			} else if errors.Is(err, Termination) {
				g.Stop()
				return
			} else {
				log.Printf("failed to update: %s", err)
			}
		}

		g.app.QueueUpdateDraw(func() {
			g.Draw()
		})

		if end {
			return
		}
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
