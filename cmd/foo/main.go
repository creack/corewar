package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"go.creack.net/corewar/asm"
	"go.creack.net/corewar/asm/parser"

	// Dump of the sources.
	//
	// find . -name '*.s'  | \
	//   while read l; do
	//     s=$(md5sum $l | sed 's/ .*//');
	//     f=$(echo $l | sed 's/.*\///');
	//     cp $l ../champions-dump/src/${f/.s/}+++$s.s;
	//   done
	dump "go.creack.net/corewar/champions-dump"
)

func compile(inputName, inputData string) ([]byte, *parser.Parser, error) {
	// Parse the input.
	_, pr, err := asm.Compile(inputName, inputData, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile: %w", err)
	}

	program, err := pr.Encode()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode program: %w", err)
	}

	return program, pr.Parser, nil

}

func md5sum(data []byte) string {
	h := md5.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func test1() error {
	d, err := dump.SrcFiles.ReadDir("srcs")
	if err != nil {
		return fmt.Errorf("read embedded dir: %w", err)
	}
	for _, f := range d {
		if !strings.HasSuffix(f.Name(), ".s") {
			continue
		}
		fmt.Printf(">> %q\n", f.Name())
		data, err := dump.SrcFiles.ReadFile("srcs/" + f.Name())
		if err != nil {
			return fmt.Errorf("read embedded file %q: %w", f.Name(), err)
		}
		buf, p, err := compile(f.Name(), string(data))
		if err != nil {
			fmt.Printf("Error compiling %q: %s\n", f.Name(), err)
			continue
		}
		fmt.Printf("%s\n", md5sum(buf))
		bb := bytes.NewBuffer(nil)
		for _, elem := range p.Nodes {
			fmt.Fprintf(bb, "%s\n", elem.PrettyPrint(p.Nodes))
		}
		if err := os.WriteFile("42/d/"+f.Name(), bb.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write file %q: %w", f.Name(), err)
		}
	}
	return nil
}

func test() error {
	d, err := dump.SrcFiles.ReadDir("src")
	if err != nil {
		return fmt.Errorf("read embedded dir: %w", err)
	}

	// Only consider the uniq entries.
	srcIdx := map[string][]string{} // md5sum -> file name.
	for _, f := range d {
		tmp := strings.Split(f.Name(), "+++")
		if len(tmp) != 2 {
			log.Printf("Warning: invalid file name %q", f.Name())
			continue
		}
		srcIdx[tmp[1]] = append(srcIdx[tmp[1]], f.Name())
		if strings.HasPrefix(f.Name(), "abel") {
			fmt.Printf("Found: %q - %q\n", f.Name(), tmp[1])
		}
	}

	srcIdxKeys := slices.Collect(maps.Keys(srcIdx))
	sort.Strings(srcIdxKeys)

	for _, k := range srcIdxKeys {
		v := srcIdx[k]
		if len(v) > 1 {
			fmt.Printf("Warning: duplicate file name %q -> %q\n", k, v)
		}
	}

	resultIdx := map[string]string{}        // md5sum prog -> file name.
	astIndex := map[string]*parser.Parser{} // full file name -> ast.
	success, failed := 0, 0
	for _, k := range srcIdxKeys {
		fullName := srcIdx[k][0]

		// Read the file.
		data, err := dump.SrcFiles.ReadFile("src/" + fullName)
		if err != nil {
			return fmt.Errorf("read embedded file %q: %w", fullName, err)
		}
		buf, p, err := compile(fullName, string(data))
		if err != nil {
			failed++
			// fmt.Printf("Error compiling %q: %s\n", fullName, err)
		} else {
			// fmt.Printf("Success compiling %q: %s\n", fullName, md5sum(buf))
			resultIdx[md5sum(buf)] = fullName
			success++
			astIndex[fullName] = p
		}
	}
	fmt.Printf("Success: %d, Failed: %d, uniq success: %d\n", success, failed, len(resultIdx))

	resultIdxKeys := slices.Collect(maps.Keys(resultIdx))
	sort.Strings(resultIdxKeys)

	bb := bytes.NewBuffer(nil)
	nameIdx := map[string]string{} // Short name -> full name.
	for _, k := range resultIdxKeys {
		v := resultIdx[k]
		nameIdx[strings.Split(v, "+++")[0]+"==="+k] = v
	}

	nameIdxKeys := slices.Collect(maps.Keys(nameIdx))
	sort.Strings(nameIdxKeys)

	fmt.Printf("uniq uniq: %d\n", len(nameIdx))

	for _, k := range nameIdxKeys {
		v := nameIdx[k]
		fmt.Fprintf(bb, "%s\n", k)

		p := astIndex[v]
		if p == nil {
			return fmt.Errorf("failed to find AST for %q", v)
		}
		buf := bytes.NewBuffer(nil)
		for _, elem := range p.Nodes {
			fmt.Fprintf(buf, "%s\n", elem.PrettyPrint(p.Nodes))
		}

		if err := os.WriteFile("champions-dump/clean-srcs/"+k+".s", buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write file %q: %w", k, err)
		}
	}

	fmt.Printf("Done.\n")
	return nil
}

func main() {
	log.SetOutput(io.Discard)
	if err := test(); err != nil {
		log.Fatal("Fail:", err.Error())
	}
}
