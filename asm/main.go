package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"unsafe"

	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/op"
)

const sample = `
.name "sebc"
.comment "sebc"
ld %:l1, r2
l1:live %3
`

const sample1 = `
.name "zork"
.comment "just a basic living prog"
.extend

l2:	sti	r1,%:live,%0x1
	and	r1,%0,r1

live:	live	%1
	zjmp	%:live

.code 42 DE AD C0 DE 12 34 61 34 61 23 61
`

// ;# Executable compile (after header):
// ;#
// ;# 0x0b,0x68,0x01,0x00,0x0f,0x00,0x01
// ;# 0x06,0x64,0x01,0x00,0x00,0x00,0x00,0x01
// ;# 0x01,0x00,0x00,0x00,0x01
// ;# 0x09,0xff,0xfb

func test0(input, output string) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	if output == "" {
		output = strings.ReplaceAll(input, ".s", ".cor")
	}
	//data = []byte(sample)
	p := parser.NewParser(input, string(data))
	if err := p.Parse(); err != nil {
		return fmt.Errorf("failed to parse: %w", err)
	}

	pr := parser.NewProgram(p)
	program, err := pr.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode program: %w", err)
	}

	for _, elem := range p.Nodes {
		log.Println(elem.PrettyPrint(p.Nodes))
	}

	header := op.ChampionHeader{
		Magic:    op.CorewarExecMagic,
		ProgSize: uint32(pr.Size()),
	}
	name := p.GetDirective("name")
	if name == "" {
		return fmt.Errorf("missing program name")
	}
	copy(header.ProgName[:], []byte(name))
	copy(header.Comment[:], []byte(p.GetDirective("comment")))

	buf := bytes.NewBuffer(nil)

	if true {
		// Write the magic number.
		tmp := make([]byte, 4)
		binary.BigEndian.PutUint32(tmp, header.Magic)
		buf.Write(tmp)

		// Make sure the field are aligned in memory.
		alignOf := int(unsafe.Alignof(header))
		// Write the program name.
		buf.Write(header.ProgName[:])
		// Pad alginment.
		if m := len(header.ProgName) % alignOf; m != 0 {
			buf.Write(make([]byte, alignOf-m))
		}

		// Write the program size.
		binary.BigEndian.PutUint32(tmp, header.ProgSize)
		buf.Write(tmp)

		// Write the program comments.
		buf.Write(header.Comment[:])
		// Pad alginment.
		if m := len(header.Comment) % alignOf; m != 0 {
			buf.Write(make([]byte, alignOf-m))
		}
	}
	// Write the main program code.
	buf.Write(program)

	if err := os.WriteFile(output, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// TODO:
// - Support Hex/Octal/Binary for numbers with 0x 0o 0b prefixes.
// - Support arithmetic expressions for numbers/labels.
//   - Death.s
//   - Backward.s
//
// - Find out and support .extend and .code
//   - .code takes hex bytes as input, not a string
//
// - Try to decompile Torpille.cor to see where t2: is supposed to be.
//
// - Support indirect label refs.
// - Debug `st r1,19` yieling encoding of 0x40 instead of 0x70
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
	n := 0xffc1
	log.Printf("%d -- %x\n", int16(n), uint16(n-400))
	log.SetFlags(0)
	output := flag.String("o", "", "output file, default to <input>.cor")
	flag.Parse()
	input := flag.Arg(0)
	if input == "" {
		fmt.Fprintf(os.Stderr, "usage: %s <input> [options]\n", os.Args[0])
		flag.PrintDefaults()
		return
	}
	if err := test0(input, *output); err != nil {
		log.Fatalf("fail: %s.", err)
	}
}
