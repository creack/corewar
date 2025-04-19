package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"go.creack.net/corewar/asm"
	"go.creack.net/corewar/asm/parser"
	"go.creack.net/corewar/assets"
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

func disam(binData []byte, strict bool) error {
	prog := &parser.Program{}

	p, err := prog.Decode(binData, strict)
	if err != nil {
		return fmt.Errorf("failed to decode program: %w", err)
	}

	// Get the program part (i.e., code after headers) to get the md5.
	buf, err := prog.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode program: %w", err)
	}

	exsitingSrc, err := searchExistingSrc(assets.CleanSrcsTargz, md5sum(buf))
	if err != nil {
		return fmt.Errorf("failed to unpack srcs: %w", err)
	}
	if exsitingSrc == nil {
		// If we didn't find a match, dump the disassembly.
		for _, elem := range p.Nodes {
			fmt.Printf("%s\n", elem.PrettyPrint(p.Nodes))
		}
		return nil
	}
	log.Printf("Found match in known sources.\n")

	_, pr2, err := asm.Compile("known-srcs", string(exsitingSrc), strict)
	if err != nil {
		// Should not happen.
		return fmt.Errorf("failed to decode known srcs: %w", err)
	}
	p2 := pr2.Parser
	actualName := p.GetDirective("name")
	actualComment := p.GetDirective("comment")
	for _, elem := range p2.Nodes {
		if d, ok := elem.(*parser.Directive); ok {
			if d.Name == "name" {
				d.Value = actualName
			} else if d.Name == "comment" {
				d.Value = actualComment
			}
		}
		fmt.Printf("%s\n", elem.PrettyPrint(p2.Nodes))
	}
	return nil
}

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
	}
	if err := disam(binData, *strict); err != nil {
		println("Fail:", err.Error())
		return
	}
	println("success")
}
