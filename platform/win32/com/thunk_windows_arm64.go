package com

import (
	"encoding/binary"
	"fmt"
)

// floatThunkCode returns the arm64 thunk. In the AAPCS64 convention integers
// and floats count separately: this is in x0, the doubles in d0, d1, ..., and
// the integer arguments after them in x1, x2, .... The callback wants
// everything in x registers in order, so the integer arguments move up by
// floats registers, then each dn goes to x(1+n).
func floatThunkCode(target uintptr, floats int) ([]byte, error) {
	if floats < 1 || floats > 6 {
		return nil, fmt.Errorf("com: float thunk for %d doubles", floats)
	}
	var ins []uint32
	// mov x(i+floats), x(i), highest first so that nothing is overwritten
	// before it moved. x1..x7 hold arguments.
	for i := 7 - floats; i >= 1; i-- {
		ins = append(ins, movX(uint32(i+floats), uint32(i)))
	}
	for n := range floats {
		// fmov x(1+n), d(n)
		ins = append(ins, 0x9E660000|uint32(n)<<5|uint32(1+n))
	}
	return appendBranch(ins, target), nil
}

// movX is mov xd, xm, an alias of orr xd, xzr, xm.
func movX(d, m uint32) uint32 { return 0xAA0003E0 | m<<16 | d }

// appendBranch appends ldr x16, =target; br x16 and the literal, aligned to
// 8 bytes. x16 is the intra-procedure-call scratch register.
func appendBranch(ins []uint32, target uintptr) []byte {
	if len(ins)%2 != 0 {
		ins = append(ins, 0xD503201F) // nop
	}
	ins = append(ins,
		0x58000050, // ldr x16, #8
		0xD61F0200, // br x16
	)
	var code []byte
	for _, w := range ins {
		code = binary.LittleEndian.AppendUint32(code, w)
	}
	return binary.LittleEndian.AppendUint64(code, uint64(target))
}

// floatCallerCode returns the arm64 caller: x1..x(floats) go to d0.., and the
// integer arguments after them move down to x1.., as a C caller passes them.
func floatCallerCode(target uintptr, floats int) ([]byte, error) {
	if floats < 1 || floats > 6 {
		return nil, fmt.Errorf("com: float caller for %d doubles", floats)
	}
	var ins []uint32
	for n := range floats {
		ins = append(ins, 0x9E670000|uint32(1+n)<<5|uint32(n)) // fmov d(n), x(1+n)
	}
	for i := 1; i+floats <= 7; i++ {
		ins = append(ins, movX(uint32(i), uint32(i+floats)))
	}
	for i := 8 - floats; i <= 7; i++ {
		ins = append(ins, movX(uint32(i), 31)) // mov x(i), xzr
	}
	return appendBranch(ins, target), nil
}
