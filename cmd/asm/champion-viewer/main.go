package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/rivo/tview"

	"go.creack.net/corewar/asm"
	"go.creack.net/corewar/op"
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

func dump(vm []byte, pc uint32) string {
	out := &strings.Builder{}

	const width = 16
	zeryBuf := make([]byte, width) // Used to compare lines.

	colors := map[string]int{}

	for _, elem := range []string{
		"magic",
		"name",
		"size",
		"comment",
	} {
		colors[elem] = nextColor()
	}

	headerSize, nameFieldSize, commentFieldSize := (op.Header{}).StructSize()
	_ = commentFieldSize
	vm = vm[:headerSize]

	nameLen := cStrLen(vm[4:])
	commentLen := cStrLen(vm[4+nameFieldSize+4:])

	_ = commentLen
	colorCode := ""
	selectedCode := ""
	i := 0
	firstEmpty := false
	for i < len(vm) {
		selectedCode = ""
		colorCode = ""
		switch {
		case i >= 0 && i < 4:
			colorCode = colorCodef(colors["magic"])
		case i >= 4 && i < 4+nameLen:
			colorCode = colorCodeModif(colors["name"], 1)
		case i >= 4+nameLen && i < 4+nameFieldSize:
			colorCode = colorCodeModif(colors["name"], 2)
		case i >= 4+nameFieldSize && i < 4+nameFieldSize+4:
			colorCode = colorCodef(colors["size"])
		case i >= 4+nameFieldSize+4 && i < 4+nameFieldSize+4+commentLen:
			colorCode = colorCodeModif(colors["comment"], 1, 4)
		case i >= 4+nameFieldSize+4+commentLen && i < 4+nameFieldSize+4+commentFieldSize:
			colorCode = colorCodeModif(colors["comment"], 2)
		}

		b := vm[i]
		if i%width == 0 {
			if bytes.Equal(vm[i:i+width], zeryBuf) {
				if !firstEmpty {
					firstEmpty = true
				} else {
					fmt.Fprintf(out, "\n*")
					for ; i < len(vm) && bytes.Equal(vm[i:i+width], zeryBuf); i += width {
					}
					firstEmpty = false
					continue
				}
			}
			if i != 0 {
				fmt.Fprintf(out, "\n")
			}
			fmt.Fprintf(out, "0x%04x", i)
		}
		if i%(width/2) == 0 {
			fmt.Fprintf(out, " ")
		}
		if i == int(pc) {
			selectedCode = "\033[7m"
		}
		fmt.Fprintf(out, " %s%s%02x\033[0m", colorCode, selectedCode, b)

		i++
	}
	for i%width != 0 {
		i++
	}
	fmt.Fprintf(out, "\n0x%04x", i)

	return out.String()
}

func run(input, output string, strict, prettyPrint bool) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	buf, pr, err := asm.Compile(input, string(data), strict)
	if err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}
	_ = pr

	// fmt.Println(dump(buf, 0))
	// return nil
	render(data, buf)
	return nil
}

func render(intput, buf []byte) {
	header := dump(buf, 5)

	newTextView := func(text string) *tview.TextView {
		return tview.NewTextView().
			SetDynamicColors(true).
			SetText(text)
	}

	rightContent := newTextView("")
	tview.ANSIWriter(rightContent).Write([]byte(header))

	leftContent := newTextView("")
	tview.ANSIWriter(leftContent).Write([]byte(intput))

	top := tview.NewFlex()
	top.SetBorder(true).SetTitle("Top")
	top.AddItem(rightContent, 0, 1, false)

	left := tview.NewFlex()
	left.SetBorder(true)
	left.SetTitle("Left (1/2 x width of Top)")
	left.AddItem(leftContent, 0, 1, false)

	flex := tview.NewFlex().
		AddItem(left, 0, 1, false).
		AddItem(top, 0, 1, false)

	app := tview.NewApplication().SetRoot(flex, true).SetFocus(flex).EnableMouse(true)
	if err := app.Run(); err != nil {
		panic(err)
	}
}

// TODO:
//
// - Try to decompile Torpille.cor to see where t2: is supposed to be.
//
// Test case:
//
// label full number, label start with number with text suffix.
// no label
// dup consecutive labels
// dup separate labels
//
// start with label
// start without label
// label .code
func main() {
	log.SetFlags(0)
	output := flag.String("o", "", "output file, default to <input>.cor")
	strict := flag.Bool("strict", false, "strict mode")
	prettyPrint := flag.Bool("pretty", false, "pretty print, do not output compiled file")
	flag.Parse()
	input := flag.Arg(0)
	if input == "" {
		tmp := strings.Split(os.Args[0], "/")
		binName := tmp[len(tmp)-1]
		fmt.Fprintf(os.Stderr, "usage: %s <.s path> [options]\n", binName)
		flag.PrintDefaults()
		return
	}
	if *output == "" {
		*output = strings.ReplaceAll(input, ".s", ".cor")
	}

	if err := run(input, *output, *strict, *prettyPrint); err != nil {
		log.Fatalf("fail: %s.", err)
	}
}
