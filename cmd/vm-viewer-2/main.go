package main

import (
	"image/color"
	"log"
	"strings"

	"github.com/hajimehoshi/bitmapfont/v3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"go.creack.net/corewar/cli"
	"go.creack.net/corewar/vm"
)

var fontFace = text.NewGoXFace(bitmapfont.Face)

// func NewGame(ctx context.Context, cw *vm.Corewar) *Game {
// 	app := tview.NewApplication().EnableMouse(true)

// 	newTextView := func(text string) *tview.TextView {
// 		return tview.NewTextView().
// 			SetDynamicColors(true).
// 			SetText(text)
// 	}

// 	//ramView := newTextView("RAM")
// 	// tview.ANSIWriter(leftContent).Write([]byte(intput))
// 	ramView := tview.NewTable().SetBorders(false)

// 	logsView := newTextView("")
// 	logsView.SetTitle("Logs").SetBorder(true)
// 	logsView.ScrollToEnd()

// 	processListView := tview.NewTable().SetBorders(false)
// 	processListView.SetTitle("Processes").SetBorder(true)

// 	stateView := newTextView("Settings")
// 	stateView.SetTitle("Settings").SetBorder(true)

// 	playersListView := tview.NewList()
// 	playersListView.SetBorder(true)
// 	playersListView.SetTitle("Players")
// 	playersListView.SetSelectedFocusOnly(true)

// 	rightPane := tview.NewFlex().SetDirection(tview.FlexRow)
// 	rightPane.
// 		AddItem(stateView, 0, 2, false).
// 		AddItem(playersListView, 0, 2, false).
// 		AddItem(logsView, 0, 3, false).
// 		AddItem(processListView, 0, 4, false)

// 	ramPane := tview.NewFlex()
// 	ramPane.SetBorder(true)
// 	ramPane.SetTitle("RAM")
// 	ramPane.AddItem(ramView, 0, 1, false)

// 	flex := tview.NewFlex().
// 		AddItem(ramPane, 0, 3, true).
// 		AddItem(rightPane, 0, 1, false)
// 	_ = flex

// 	pages := tview.NewPages()
// 	pages.AddPage("main", flex, true, true)

// 	for _, p := range cw.Players {
// 		playersListView.AddItem("", "", 0, func() {
// 			pages.ShowPage(fmt.Sprintf("disasm-player-%d", p.Number))
// 		})
// 	}

// 	ctx, cancel := context.WithCancel(ctx)

// 	return &Game{
// 		app: app,

// 		root: pages,

// 		mainPage:        flex,
// 		ramView:         ramView,
// 		processListView: processListView,
// 		stateView:       stateView,
// 		playerListView:  playersListView,
// 		logsView:        logsView,

// 		cw:     cw,
// 		ctx:    ctx,
// 		cancel: cancel,

// 		paused: true,
// 	}
// }

// type Game struct {
// 	app *tview.Application

// 	root *tview.Pages

// 	mainPage *tview.Flex

// 	ramView         tview.Primitive
// 	processListView *tview.Table
// 	stateView       tview.Primitive
// 	playerListView  tview.Primitive
// 	logsView        *tview.TextView

// 	cw *vm.Corewar

// 	paused   bool
// 	pausedMu sync.Mutex

// 	nextStep   bool
// 	nextStepMu sync.Mutex

// 	ctx    context.Context
// 	cancel context.CancelFunc
// }

// var Termination = errors.New("termination")

// func (g *Game) Stop() {
// 	g.app.Stop()
// 	g.cancel()
// }

// func (g *Game) Init() {
// 	f := func(event *tcell.EventKey) *tcell.EventKey {
// 		curPage, _ := g.root.GetFrontPage()
// 		switch event.Key() {
// 		case tcell.KeyCtrlC, tcell.KeyEscape:
// 			if curPage != "main" {
// 				g.root.SwitchToPage("main")
// 				return nil
// 			}
// 			g.Stop()
// 			return nil
// 		case tcell.KeyEnter:
// 			if curPage != "main" {
// 				g.root.SwitchToPage("main")
// 				return nil
// 			}
// 			return event
// 		}
// 		switch event.Rune() {
// 		case 'n':
// 			g.nextStepMu.Lock()
// 			g.nextStep = true
// 			g.nextStepMu.Unlock()
// 			return nil
// 		case ' ':
// 			if curPage == "main" {
// 				g.pausedMu.Lock()
// 				g.paused = !g.paused
// 				g.pausedMu.Unlock()
// 			} else {
// 				g.root.SwitchToPage("main")
// 			}
// 			return nil
// 		case 'q':
// 			if curPage != "main" {
// 				g.root.SwitchToPage("main")
// 				return nil
// 			}
// 			g.Stop()
// 			return nil
// 		}
// 		return event
// 	}
// 	g.root.SetInputCapture(f)
// 	go func() {
// 	loop:
// 		select {
// 		case msg := <-g.cw.Messages:
// 			g.app.QueueUpdateDraw(func() {
// 				if msg.Type == vm.MsgClear {
// 					g.logsView.Clear()
// 					return
// 				}
// 				if msg.Type == vm.MsgPause {
// 					g.pausedMu.Lock()
// 					g.paused = true
// 					g.pausedMu.Unlock()
// 					return
// 				}
// 				// NOTE: Seems like there is a bug with tview, we can't reset the color to default
// 				// with [:] or [:::], so we use tcell default.
// 				colorCode := "[" + tcell.ColorDefault.String() + ":::]"
// 				if msg.Process != nil {
// 					colorCode = "[" + colors[msg.Process.ID%len(colors)].String() + ":::]"
// 				}
// 				if msg.Process != nil {
// 					fmt.Fprintf(g.logsView, "%s[%d] %s[:::]\n", colorCode, msg.Process.ID, strings.TrimSuffix(msg.Message, "\n"))
// 				} else {
// 					fmt.Fprintf(g.logsView, "%s%s[:::]\n", colorCode, strings.TrimSuffix(msg.Message, "\n"))
// 				}
// 			})
// 			g.app.Draw()
// 		case <-g.ctx.Done():
// 			return
// 		}
// 		goto loop
// 	}()
// }

// func (g *Game) Update() error {
// 	isPaused := func() bool {
// 		g.pausedMu.Lock()
// 		defer g.pausedMu.Unlock()
// 		return g.paused
// 	}
// 	forceNextStep := func() bool {
// 		g.nextStepMu.Lock()
// 		defer g.nextStepMu.Unlock()
// 		if g.nextStep {
// 			g.nextStep = false
// 			return true
// 		}
// 		return false
// 	}
// 	if !forceNextStep() && isPaused() {
// 		return nil
// 	}

// 	if err := g.cw.Round(); err != nil {
// 		return fmt.Errorf("failed to execute instruction: %w", err)
// 	}
// 	g.Draw()

// 	return nil
// }

// func (g *Game) drawProcessList() {
// 	g.processListView.SetTitle(fmt.Sprintf("Processes (%d)", len(g.cw.Processes)))
// 	g.processListView.Clear()
// 	for i, elem := range []string{
// 		"pid",
// 		"ppid",
// 		"pc",
// 		"op",
// 		"wait",
// 		"registers",
// 		"carry",
// 	} {
// 		cell := tview.NewTableCell(elem).
// 			SetAttributes(tcell.AttrBold).
// 			SetAlign(tview.AlignCenter)

// 		g.processListView.SetCell(0, i, cell).SetFixed(1, i)
// 	}

// 	dumpRegisters := func(regs []uint32) string {
// 		parts := make([]string, 0, len(regs))
// 		for _, elem := range regs {
// 			val := "."
// 			if elem != 0 {
// 				val = "x"
// 			}
// 			parts = append(parts, val)
// 		}
// 		return strings.Join(parts, "")
// 	}
// 	for i, elem := range g.cw.Processes {
// 		curInsName := ""
// 		if elem.CurInstruction != nil {
// 			curInsName = elem.CurInstruction.OpCode.Name
// 		}
// 		for j, content := range []any{
// 			elem.ID,
// 			elem.Player.Number,
// 			fmt.Sprintf("%04x", elem.PC),
// 			curInsName,
// 			elem.WaitCycles,
// 			dumpRegisters(elem.Registers[:]),
// 			elem.Carry,
// 		} {
// 			cell := tview.NewTableCell(fmt.Sprint(content)).SetAlign(tview.AlignRight)
// 			cell.SetTextColor(colors[elem.ID%len(colors)])
// 			g.processListView.SetCell(i+1, j, cell)

// 		}
// 	}
// }

// func (g *Game) drawPlayerList() {
// 	pv := g.playerListView.(*tview.List)

// 	for i, p := range g.cw.Players {
// 		deadCode := ""
// 		if p.Dead {
// 			deadCode = "s"
// 		}
// 		pid := p.Number
// 		attr := "[" + colors[pid%len(colors)].String() + "::" + deadCode + ":]"
// 		txt := fmt.Sprintf("%s[%d] %s (%d)[:::]", attr, p.Number, p.Name, p.ProcessCount)
// 		pv.SetItemText(i, txt, "")
// 	}
// }

// func (g *Game) drawState() {
// 	sv := g.stateView.(*tview.TextView)
// 	sv.Clear()

// 	fmt.Fprintf(sv, "Cycles: %d\n", g.cw.Cycle)
// 	fmt.Fprintf(sv, "Current CyclesToDie: %d\n", g.cw.CurCyclesToDie)
// 	fmt.Fprintf(sv, "Next CyclesToCheck: %d\n", g.cw.Config.CyclesToDie)
// 	fmt.Fprintf(sv, "Memory Size: %d\n", g.cw.Config.MemSize)
// 	fmt.Fprintf(sv, "IdxMod: %d\n", g.cw.Config.IdxMod)
// 	fmt.Fprintf(sv, "NumLives: %d\n", g.cw.Config.NumLives)
// 	fmt.Fprintf(sv, "CycleDelta: %d\n", g.cw.Config.CycleDelta)
// 	fmt.Fprintf(sv, "Period live count: %d\n", g.cw.LiveCalls)
// }

// func (g *Game) drawRAM() {
// 	const width = 64
// 	ramView := g.ramView.(*tview.Table)
// 	ramView.SetSelectable(true, true)
// 	for i, elem := range g.cw.Ram {
// 		onClick := []func(){func() { g.cw.Messages <- vm.NewMessage(vm.MsgPause, nil, "") }}

// 		cell := tview.NewTableCell(fmt.Sprintf("%02x", elem.Value))
// 		if elem.Process != nil {
// 			cell.SetTextColor(colors[elem.Process.ID%len(colors)])
// 			if elem.AccessType == 1 {
// 				cell.SetAttributes(tcell.AttrBold)
// 			} else if elem.AccessType == 2 {
// 				cell.SetAttributes(tcell.AttrItalic | tcell.AttrDim)
// 			} else if elem.AccessType == 3 {
// 				cell.SetAttributes(tcell.AttrItalic | tcell.AttrDim | tcell.AttrUnderline | tcell.AttrBlink)
// 			}
// 			onClick = append(onClick, func() {
// 				g.cw.Messages <- vm.NewMessage(vm.MsgDebug, elem.Process, fmt.Sprintf("Yup!!! %d", elem.Process.ID))
// 			})
// 		} else if elem.Value == 0 {
// 			cell.SetTextColor(tcell.ColorDimGray)
// 			cell.SetAttributes(tcell.AttrDim)
// 		}
// 		for _, p := range g.cw.Processes {
// 			if !p.Player.Dead && i == int(p.PC) {
// 				cell.SetAttributes(tcell.AttrReverse).SetTextColor(colors[p.ID%len(colors)])
// 				onClick = append(onClick, func() {
// 					g.cw.Messages <- vm.NewMessage(vm.MsgDebug, p, fmt.Sprintf("PC player %d", p.Player.Number))
// 				})
// 			}
// 		}
// 		cell.SetClickedFunc(func() bool {
// 			go func() {
// 				for _, f := range onClick {
// 					f()
// 				}
// 			}()
// 			return true
// 		})
// 		ramView.SetCell(i/width, i%width, cell)
// 	}

// 	// ramView := g.ramView.(*tview.TextView)
// 	// ramView.Clear()
// 	// w := tview.ANSIWriter(ramView)
// 	// io.Copy(w, strings.NewReader(dump(g.cw.Ram, g.cw.Processes)))
// }

// func (g *Game) Draw() {
// 	g.drawRAM()
// 	g.drawState()
// 	g.drawPlayerList()
// 	g.drawProcessList()
// }

const initialScreenWidth, initialScreenHeight = 1024, 768

// Game implements ebiten.Game interface.
type Game struct {
	fontFace *text.Face
}

// Update proceeds the game state.
// Update is called every tick (1/60 [s] by default).
func (g *Game) Update() error {
	// Write your game's logical update.
	return nil
}

// Draw draws the game screen.
// Draw is called every frame (typically 1/60[s] for 60Hz display).
func (g *Game) Draw(screen *ebiten.Image) {
	keyStrs := []string{"a", "b", "c"}
	keyNames := []string{"A", "B", "C"}
	// Write your game's rendering.
	textOp := &text.DrawOptions{}
	textOp.LineSpacing = fontFace.Metrics().HLineGap + fontFace.Metrics().HAscent + fontFace.Metrics().HDescent
	textOp.ColorScale.ScaleWithColor(color.RGBA{R: 255})

	text.Draw(screen, strings.Join(keyStrs, ", ")+"\n"+strings.Join(keyNames, ", "), fontFace, textOp)
}

// Layout takes the outside size (e.g., the window size) and returns the (logical) screen size.
// If you don't have to adjust the screen size with the outside size, just return a fixed size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return initialScreenWidth, initialScreenHeight
}

func main() {
	game := &Game{}
	// Specify the window size as you like. Here, a doubled size is specified.
	ebiten.SetWindowSize(initialScreenWidth, initialScreenHeight)
	ebiten.SetWindowTitle("Corewar")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	cfg, _, err := cli.ParseConfig()
	if err != nil {
		log.Fatalf("Failed to parse cli config: %s.", err)
	}

	cw := vm.NewCorewar(cfg)
	if err := cw.Round(); err != nil {
		log.Fatalf("Failed to execute first round: %s.", err)
	}

	// Call ebiten.RunGame to start your game loop.
	if err := ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		InitUnfocused: true,
	}); err != nil {
		log.Fatal(err)
	}
}
