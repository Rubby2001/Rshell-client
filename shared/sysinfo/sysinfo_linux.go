//go:build linux && !arm

package sysinfo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"
)

func arrayToString(x [65]int8) string {
	var buf [65]byte
	for i, b := range x {
		buf[i] = byte(b)
	}
	str := string(buf[:])
	if i := strings.Index(str, "\x00"); i != -1 {
		str = str[:i]
	}
	return str
}

func getUname() syscall.Utsname {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		fmt.Printf("Uname: %v", err)
		return syscall.Utsname{} //nil
	}
	return uname
}

func GetOSVersion() (string, error) {
	uname := getUname()

	if len(uname.Release) > 0 {
		return arrayToString(uname.Release), nil
	}
	return "0.0", errors.New("Something wrong")
}

func IsHighPriv() bool {
	if os.Getuid() == 0 {
		return true
	}
	return false
}

func IsOSX64() (bool, error) {
	uname := getUname()
	if arrayToString(uname.Machine) == "x86_64" {
		return true, nil
	}
	return false, nil
}

func GetCodePageANSI() ([]byte, error) {
	//linux also use utf8 as default
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 65001)
	return b, nil
}

func GetCodePageOEM() ([]byte, error) {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 65001)
	return b, nil
}

func GetUsername() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", nil
	}
	usr := user.Username
	return usr, nil
}

func IsPidX64(pid uint32) (bool, error) {
	/*is64 := false

	hProcess, err := windows.OpenProcess(uint32(0x1000), false, pid)
	if err != nil {
		return IsProcessX64()
	}

	_ = windows.IsWow64Process(hProcess, &is64)*/

	return true, nil
}

// return 0
func GetProcessSessionId(pid int32) uint32 {
	return 0
}
