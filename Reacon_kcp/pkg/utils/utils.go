package utils

import (
	"bytes"
	"encoding/binary"
	"github.com/xtaci/kcp-go/v5"
)

var MetaInfo []byte
var KCPClient *kcp.UDPSession

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
		//args := strings.Split(strings.TrimRight(string(arg), "\x00"), "\x00")
		//return []byte(args[0]), nil
		return arg, nil
	} else {
		return nil, err
	}

}
