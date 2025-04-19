package disasm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"strings"

	"go.creack.net/corewar/asm"
	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/assets"
	"go.creack.net/corewar/op"
)

func md5sum(data []byte) string {
	h := md5.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// search is the md5 of the program (.cor after headers).
func searchExistingSrc(targzData []byte, search string) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(targzData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = r.Close() }() // Best effort.

	// Unpack the tar archive.
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break // End of archive
			}
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}
		// If we don't have a name, skip it.
		if !strings.HasSuffix(hdr.Name, search+".s") {
			continue
		}
		// Read the file content.
		buf := bytes.NewBuffer(nil)
		if _, err := io.Copy(buf, tr); err != nil {
			return nil, fmt.Errorf("failed to read file %q: %w", hdr.Name, err)
		}
		return buf.Bytes(), nil
	}

	return nil, nil
}

func Disam(inputName string, binData []byte, strict bool) (*parser.Program, error) {
	prog := &parser.Program{}

	p, err := prog.Decode(binData, strict)
	if err != nil {
		return nil, fmt.Errorf("failed to decode program: %w", err)
	}

	// Get the program part (i.e., code after headers) to get the md5.
	buf, err := prog.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode program: %w", err)
	}

	exsitingSrc, err := searchExistingSrc(assets.CleanSrcsTargz, md5sum(buf))
	if err != nil {
		return nil, fmt.Errorf("failed to unpack srcs: %w", err)
	}
	if exsitingSrc == nil {
		// If we didn't find a match, return what we have.
		return prog, nil
	}
	// log.Printf("Found match in known sources for %q.\n", inputName)

	_, pr2, err := asm.Compile("known-srcs", string(exsitingSrc), strict)
	if err != nil {
		// Should not happen.
		return nil, fmt.Errorf("failed to decode known srcs: %w", err)
	}
	p2 := pr2.Parser
	actualName := p.GetDirective(op.NameCmdString)
	actualComment := p.GetDirective(op.CommentCmdString)
	for _, elem := range p2.Nodes {
		if d, ok := elem.(*parser.Directive); ok {
			if string(op.DirectiveChar)+d.Name == op.NameCmdString {
				d.Value = actualName
			} else if string(op.DirectiveChar)+d.Name == op.CommentCmdString {
				d.Value = actualComment
			}
		}
	}
	return pr2, nil
}
