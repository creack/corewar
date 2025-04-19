package parser

import "go.creack.net/corewar/op"

type Label struct {
	Name string
}

func (l *Label) PrettyPrint(nodes []Node) string {
	// Unless we are immediately after a label, prefix with a newline.
	var prev Node
	for _, n := range nodes {
		if l1, ok := n.(*Label); ok && l1 == l {
			if _, ok := prev.(*Label); ok {
				return l.Name + string(op.LabelChar)
			}
			return "\n" + l.Name + string(op.LabelChar)
		}
		prev = n
	}
	// Should never happen.
	panic("self reference not found in nodes")
}

func (l Label) Encode(p *Program) ([]byte, error) {
	p.labels[l.Name] = p.idx
	return nil, nil
}
