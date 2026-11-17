package com

import (
	"encoding/binary"
	"fmt"
)

// floatThunkCode returns the amd64 thunk. In the Windows x64 convention an
// argument has a register by position: the second is RDX or XMM1, the third
// R8 or XMM2, the fourth R9 or XMM3. So the doubles move from XMMn to the
// integer register of the same position, and the rest stay where they are.
func floatThunkCode(target uintptr, floats int) ([]byte, error) {
	if floats < 1 || floats > 3 {
		return nil, fmt.Errorf("com: float thunk for %d doubles", floats)
	}
	gpr := [...]byte{2, 8, 9} // RDX, R8, R9
	var code []byte
	for i := range floats {
		xmm, rm := byte(1+i), gpr[i]
		// movq r64, xmm: 66 REX.W(+B) 0F 7E /r, with xmm in reg and the
		// integer register in r/m.
		rex := byte(0x48)
		if rm >= 8 {
			rex |= 0x01
		}
		code = append(code, 0x66, rex, 0x0F, 0x7E, 0xC0|xmm<<3|rm&7)
	}
	code = appendJump(code, target)
	return code, nil
}

// appendJump appends mov rax, target; jmp rax. RAX holds no argument.
func appendJump(code []byte, target uintptr) []byte {
	code = append(code, 0x48, 0xB8)
	code = binary.LittleEndian.AppendUint64(code, uint64(target))
	return append(code, 0xFF, 0xE0)
}

// floatCallerCode returns the amd64 caller: the integer registers at the
// positions of the doubles go to XMM1.., and are zeroed.
func floatCallerCode(target uintptr, floats int) ([]byte, error) {
	if floats < 1 || floats > 3 {
		return nil, fmt.Errorf("com: float caller for %d doubles", floats)
	}
	gpr := [...]byte{2, 8, 9} // RDX, R8, R9
	var code []byte
	for i := range floats {
		xmm, rm := byte(1+i), gpr[i]
		rex := byte(0x48)
		if rm >= 8 {
			rex |= 0x01
		}
		// movq xmm, r64: 66 REX.W(+B) 0F 6E /r
		code = append(code, 0x66, rex, 0x0F, 0x6E, 0xC0|xmm<<3|rm&7)
		// xor r32, r32 clears the whole register.
		if rm >= 8 {
			code = append(code, 0x45)
		}
		code = append(code, 0x31, 0xC0|(rm&7)<<3|rm&7)
	}
	return appendJump(code, target), nil
}
