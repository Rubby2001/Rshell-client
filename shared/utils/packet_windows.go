//go:build windows

package utils

import (
	"unicode/utf8"
	"golang.org/x/sys/windows"
)

const (
	cpACP  = 0
	cpUTF8 = 65001
)

func codepageToUTF8(b []byte) ([]byte, error) {
	if utf8.Valid(b) {
		return b, nil
	}
	utf16, err := gbkToUTF16(b)
	if err != nil {
		return b, nil
	}
	return utf16ToUTF8(utf16), nil
}

func gbkToUTF16(b []byte) ([]uint16, error) {
	if len(b) == 0 {
		return nil, nil
	}
	n, err := windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), nil, 0)
	if n == 0 {
		return nil, err
	}
	utf16 := make([]uint16, n)
	_, err = windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), &utf16[0], n)
	if err != nil {
		return nil, err
	}
	return utf16, nil
}

func utf16ToUTF8(utf16 []uint16) []byte {
	var buf []byte
	for _, r := range utf16 {
		if r < 0x80 {
			buf = append(buf, byte(r))
		} else if r < 0x800 {
			buf = append(buf, byte(0xC0|(r>>6)), byte(0x80|(r&0x3F)))
		} else {
			buf = append(buf, byte(0xE0|(r>>12)), byte(0x80|((r>>6)&0x3F)), byte(0x80|(r&0x3F)))
		}
	}
	return buf
}

func utf8ToUTF16(utf8Data []byte) []uint16 {
	runes := []rune(string(utf8Data))
	utf16 := make([]uint16, 0, len(runes))
	for _, r := range runes {
		if r < 0x10000 {
			utf16 = append(utf16, uint16(r))
		} else {
			r -= 0x10000
			utf16 = append(utf16, uint16(0xD800+((r>>10)&0x3FF)), uint16(0xDC00+(r&0x3FF)))
		}
	}
	return utf16
}
