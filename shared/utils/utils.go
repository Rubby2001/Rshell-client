package utils

import (
	"bytes"
	"encoding/binary"
)

var MetaInfo []byte

func BytesCombine(pBytes ...[]byte) []byte {
	return bytes.Join(pBytes, []byte(""))
}

func ParseAnArg(buf *bytes.Buffer) ([]byte, error) {
	argLenBytes := make([]byte, 4)
	_, err := buf.Read(argLenBytes)
	if err != nil {
		return nil, err
	}
	argLen := binary.BigEndian.Uint32(argLenBytes)
	if argLen != 0 {
		arg := make([]byte, argLen)
		_, err = buf.Read(arg)
		if err != nil {
			return nil, err
		}
		return arg, nil
	}
	return nil, err
}
