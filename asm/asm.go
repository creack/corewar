package asm

import (
	"bytes"
	"fmt"

	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/op"
)

func Compile(inputName, inputData string, strict bool) ([]byte, *parser.Program, error) {
	// Parse the input.
	p := parser.NewParser(inputName, inputData)
	if err := p.Parse(); err != nil {
		return nil, nil, fmt.Errorf("failed to parse: %w", err)
	}

	// Encode the program.
	pr := parser.NewProgram(p, strict)
	program, err := pr.Encode()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode program: %w", err)
	}

	// Create the header.
	header := op.Header{
		Magic:    op.CorewarExecMagic,
		ProgSize: uint32(pr.Size()),
	}
	name := p.GetDirective(op.NameCmdString)
	if name == "" {
		return nil, nil, fmt.Errorf("missing program name")
	}
	copy(header.ProgName[:], []byte(name))
	comment := p.GetDirective(op.CommentCmdString)
	if len(comment) > op.CommentLength {
		return nil, nil, fmt.Errorf("comment exceeds maximum length")
	}
	copy(header.Comment[:], []byte(comment))

	buf := bytes.NewBuffer(nil)

	// Write the magic number.
	tmp := make([]byte, 4)
	op.Endian.PutUint32(tmp, header.Magic)
	buf.Write(tmp)

	// Make sure the field are aligned in memory.
	alignOf := 4
	// Write the program name.
	buf.Write(header.ProgName[:])
	// Pad alginment.
	if m := len(header.ProgName) % alignOf; m != 0 {
		buf.Write(make([]byte, alignOf-m))
	}

	// Write the program size.
	op.Endian.PutUint32(tmp, header.ProgSize)
	buf.Write(tmp)

	// Write the program comments.
	buf.Write(header.Comment[:])
	// Pad alginment.
	if m := len(header.Comment) % alignOf; m != 0 {
		buf.Write(make([]byte, alignOf-m))
	}

	// Write the main program code.
	buf.Write(program)
	return buf.Bytes(), pr, nil
}
