package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"go.creack.net/corewar/asm"
)

func run(input, output string, strict, prettyPrint bool) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	buf, pr, err := asm.Compile(input, string(data), strict)
	if err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}
	if prettyPrint {
		for _, elem := range pr.Nodes {
			fmt.Printf("%s\n", elem.PrettyPrint(pr.Nodes))
		}
		return nil
	}

	if err := os.WriteFile(output, buf, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
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
