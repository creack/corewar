package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"go.creack.net/corewar/disasm"
)

func main() {
	strict := flag.Bool("strict", false, "strict mode")
	flag.Parse()
	f := flag.Arg(0)
	if f == "" {
		tmp := strings.Split(os.Args[0], "/")
		binName := tmp[len(tmp)-1]
		fmt.Fprintf(os.Stderr, "usage: %s <.cor path> [options]\n", binName)
		flag.PrintDefaults()
		return
	}
	binData, err := os.ReadFile(f)
	if err != nil {
		log.Fatalf("failed to read file %q: %s", f, err)
		return
	}
	prog, err := disasm.Disam(f, binData, *strict)
	if err != nil {
		log.Fatalf("failed to disassemble %q: %s", f, err)
		return
	}
	for _, elem := range prog.Nodes {
		fmt.Printf("%s\n", elem.PrettyPrint(prog.Nodes))
	}
}
