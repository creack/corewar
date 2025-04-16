package parser

import (
	"fmt"
	"strconv"
	"strings"

	"go.creack.net/corewar/op"
)

type Directive struct {
	Name  string
	Value string
}

func (d Directive) String() string {
	return fmt.Sprintf("<%c%s %.5q...>", op.DirectiveChar, d.Name, d.Value)
}

func isLastRelevantNode(nodes []Node, n Node) bool {
	for i, node := range nodes {
		if node == n {
			for _, elem := range nodes[i+1:] {
				switch elem.(type) {
				case *Label:
				case *Directive:
				// TODO: Add *Comment if we end up adding it.
				default:
					return false
				}
			}
			return true
		}
	}
	// SHould never happen.
	panic("self reference not found in nodes")
}

func (d *Directive) PrettyPrint(nodes []Node) string {
	out := string(op.DirectiveChar) + d.Name

	// If we have a label at any point before us,
	// indent the directive, unless we are the last node.
	for _, n := range nodes {
		if _, ok := n.(*Label); ok {
			if !isLastRelevantNode(nodes, d) {
				out = "\t" + out
			} else {
				out = "\n" + out
			}
			break
		}
		if dd, ok := n.(*Directive); ok && dd == d {
			break
		}
	}
	if d.Value != "" {
		if d.Name == "code" {
			out += " " + d.Value
		} else {
			out += " \"" + d.Value + "\""
		}
	}
	return out
}

func (d Directive) Encode(p *Program) error {
	if d.Name == "extend" {
		p.extendModeEnabled = true
		return nil
	}
	if d.Name != "code" {
		// Unless we have the 'code' directive, we don't encode anything.
		return nil
	}
	if !p.extendModeEnabled {
		return fmt.Errorf(".extend must be set to use .code directive")
	}

	// Parse the raw code as hex.
	for _, elem := range strings.Split(d.Value, " ") {
		if len(elem) > 2 {
			return fmt.Errorf("code directive hex %q is too long", elem)
		}
		n, err := strconv.ParseUint(elem, 16, 8)
		if err != nil {
			return fmt.Errorf("failed to parse code directive hex %q: %w", elem, err)
		}
		p.buf[p.idx] = byte(n)
		p.idx++
	}
	return nil
}
