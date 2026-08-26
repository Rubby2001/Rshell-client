//go:build darwin

package sysinfo

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"os/user"
)

func GetOSVersion() (string, error) {
	cmd := exec.Command("sw_vers", "-productVersion")
	out, _ := cmd.CombinedOutput()
	return string(out[:]), nil
}

func IsHighPriv() bool {
	if os.Getuid() == 0 {
		return true
	}
	return false
}

func IsOSX64() (bool, error) {
	cmd := exec.Command("sysctl", "hw.cpu64bit_capable")
	out, _ := cmd.CombinedOutput()
	out = bytes.ReplaceAll(out, []byte("hw.cpu64bit_capable: "), []byte(""))
	if string(out) == "1" {
		return true, nil
	}
	return false, nil
}

func GetCodePageANSI() ([]byte, error) {
	// darwin use utf8(I guess)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 65001)
	return b, nil
}

func GetCodePageOEM() ([]byte, error) {
	//use utf8
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
