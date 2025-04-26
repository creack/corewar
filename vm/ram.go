package vm

import "go.creack.net/corewar/op"

type Ram []RamEntry

func (r Ram) Bytes(addr uint32, size int) []byte {
	out := make([]byte, size)
	for i := range size {
		out[i] = r[(int(addr)+i)%len(r)].Value
	}
	return out
}

func (r Ram) GetRamValue32(p *Process, addr uint32) uint32 {
	b := make([]byte, 4)
	for i := range 4 {
		b[i] = r[(int(addr)+i)%len(r)].Value
		r[(int(addr)+i)%len(r)].Process = p
		r[(int(addr)+i)%len(r)].AccessType = 2
	}
	return op.Endian.Uint32(b)
}

func (r Ram) GetRamValue16(p *Process, addr uint32) uint16 {
	b := make([]byte, 2)
	for i := range 2 {
		b[i] = r[(int(addr)+i)%len(r)].Value
		r[(int(addr)+i)%len(r)].Process = p
		r[(int(addr)+i)%len(r)].AccessType = 3
	}
	return op.Endian.Uint16(b)
}

// func (r Ram) GetRamValueIndex(p *Process, pc uint32, idx int16, mod int64) uint16 {
// 	start := int64(pc) + (int64(idx) % mod)

// 	b := make([]byte, 2)
// 	for i := range 2 {
// 		b[i] = r[(int64(i)+start)%int64(len(r))].Value
// 		r[(int64(i)+start)%int64(len(r))].Process = p
// 		r[(int64(i)+start)%int64(len(r))].AccessType = 2
// 	}

// 	return op.Endian.Uint16(b)
// }

func (r Ram) SetRamValue(p *Process, addr, value uint32) {
	b := make([]byte, 4)
	op.Endian.PutUint32(b, value)
	for i := range 4 {
		r[(int(addr)+i)%len(r)].Value = b[i]
		r[(int(addr)+i)%len(r)].Process = p
		r[(int(addr)+i)%len(r)].AccessType = 1
	}
}

type RamEntry struct {
	Value      byte
	Process    *Process // Who last used the entry.
	AccessType int
}
