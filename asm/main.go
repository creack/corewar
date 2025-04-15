package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"unsafe"

	parse "go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/op"
)

// TODO: Investigate why the lexer yields 2 EOF at the end.
const sample = `
.name "zork"
.comment "just a basic living prog"

live:
	sti	r1, %:live, %1 ; foo
	ldi r2,r3,r4
`

// `
// bite:	sti     r1,%:copie,%2   ; Pour le ld a l'entree
//         ldi     %:copie,%3,r2   ; met le ld a l'entree
//         sti     r2,%:entree,%-4

// copie:
// 	and	r1, %0, r1

// live:
// 	live	%1
// entree:
// 	zjmp	%:live

// ;# Executable compile (after header):
// ;#
// ;# 0x0b,0x68,0x01,0x00,0x0f,0x00,0x01
// ;# 0x06,0x64,0x01,0x00,0x00,0x00,0x00,0x01
// ;# 0x01,0x00,0x00,0x00,0x01
// ;# 0x09,0xff,0xfb

// `

func encodeProgram(p *parse.Parser, program []byte, labels map[string]int) (int, map[string]int, error) {
	// If we have labels, it means we already encoded once and have the labels index.
	// Error out if we encounter a label that we don't know
	hasLabelIndex := labels != nil
	if !hasLabelIndex {
		labels = map[string]int{}
	}

	// Keep track whether or not we have missing labels
	// so we can avoid re-encoding if not necessary.
	hasMissingLabels := false

	idx := 0
	for _, ins := range p.Instructions {
		// Has labels are indexed from the instruction start
		// keep track of it.
		idxInstruction := idx

		// If the instruction has a label, we need to store
		// it in the labels map for future reference.
		for _, label := range ins.Labels {
			labels[label] = idx
		}
		// Store the opcode and advance.
		program[idx] = ins.OpCode.Code
		idx++
		// If the instruction requires an encoding byte,
		// store it and advance.
		if ins.OpCode.EncodingByte {
			program[idx] = ins.ParamsEncoding()
			idx++
		}
		// Encore each param.
		for _, param := range ins.Params {
			// Handle case for label references.
			if strings.HasPrefix(param.Value, string(op.LabelChar)) {
				// If we already know the label, we can
				// replace the it with the offset.
				// Otherwise, keep going, it will be known the second time.
				if _, ok := labels[param.Value[1:]]; ok {
					param.Value = strconv.Itoa(labels[param.Value[1:]] - idxInstruction)
				} else if !hasLabelIndex {
					hasMissingLabels = true
				} else {
					// If we don't know the label while having
					// the labels index, error out.
					return 0, nil, fmt.Errorf("unknown label %q", param.Value)
				}
			}
			// Encode the paramter and advance the index.
			n, err := param.Encode(program[idx:], ins.OpCode.AddressMode)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to encode parameter %s: %w", param, err)
			}
			idx += n
		}
	}
	// If we don't have any missing labels, we don't need to re-encode,
	// return nil label map to indicate that.
	if !hasMissingLabels {
		return idx, nil, nil
	}
	return idx, labels, nil
}

func test0(input, output string) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	if output == "" {
		output = strings.ReplaceAll(input, ".s", ".cor")
	}
	//data = []byte(sample)
	p := parse.NewParser(input, string(data))
	if err := p.Parse(); err != nil {
		return fmt.Errorf("failed to parse: %w", err)
	}
	fmt.Printf("Parsed successfully: %v\n", p.Directives)
	log.Printf("instr: %d\n", len(p.Instructions))

	program := make([]byte, op.MemSize)
	progSize, labels, err := encodeProgram(p, program, nil)
	if err != nil {
		return fmt.Errorf("failed to encode program: %w", err)
	}
	if labels != nil {
		log.Println("reenc")
		if _, _, err := encodeProgram(p, program, labels); err != nil {
			return fmt.Errorf("failed to encode program: %w", err)
		}
	}

	for _, elem := range p.Instructions {
		log.Println(elem)
	}

	header := op.ChampionHeader{
		Magic:    op.CorewarExecMagic,
		ProgSize: uint32(progSize),
	}
	copy(header.ProgName[:], p.Directives["name"])
	copy(header.Comment[:], p.Directives["comment"])

	buf := bytes.NewBuffer(nil)

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

	// Write the main program code.
	buf.Write(program[:progSize])

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
func main() {
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
