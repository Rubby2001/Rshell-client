//go:build windows

package sysinfo

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
	"syscall"
	"unsafe"
)

var (
	Kernel32 = syscall.NewLazyDLL("Kernel32.dll")
	//Advapi32 = syscall.NewLazyDLL("Advapi32.dll")
)

type SystemInfo struct {
	ProcessorArchitecture     ProcessorArchitecture
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uint64
	NumberOfProcessors        uint32
	ProcessorType             ProcessorType
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

type ProcessorArchitecture uint16

const (
	ProcessorArchitectureAMD64   ProcessorArchitecture = 9
	ProcessorArchitectureARM     ProcessorArchitecture = 5
	ProcessorArchitectureARM64   ProcessorArchitecture = 12
	ProcessorArchitectureIA64    ProcessorArchitecture = 6
	ProcessorArchitectureIntel   ProcessorArchitecture = 0
	ProcessorArchitectureUnknown ProcessorArchitecture = 0xFFFF
)

type ProcessorType uint32

const (
	ProcessorTypeIntel386     ProcessorType = 386
	ProcessorTypeIntel486     ProcessorType = 486
	ProcessorTypeIntelPentium ProcessorType = 586
	ProcessorTypeIntelIA64    ProcessorType = 2200
	ProcessorTypeAMDX8664     ProcessorType = 8664
)

func GetOSVersion() (string, error) {
	version, err := syscall.GetVersion()
	if err != nil {
		return "", err
	}
	//fmt.Printf("%d.%d (%d)\n", byte(version), uint8(version>>8), version>>16)

	return fmt.Sprintf("%d.%d.%d\n", byte(version), uint8(version>>8), version>>16), nil
}

func IsHighPriv() bool {
	token, err := syscall.OpenCurrentProcessToken()
	defer token.Close()
	if err != nil {
		fmt.Printf("open current process token failed: %v\n", err)
		return false
	}
	/*
		ref:
		C version https://vimalshekar.github.io/codesamples/Checking-If-Admin
		Go package https://github.com/golang/sys/blob/master/windows/security_windows.go ---> IsElevated
		maybe future will use ---> golang/x/sys/windows
	*/
	var isElevated uint32
	var outLen uint32
	err = syscall.GetTokenInformation(token, syscall.TokenElevation, (*byte)(unsafe.Pointer(&isElevated)), uint32(unsafe.Sizeof(isElevated)), &outLen)
	if err != nil {
		return false
	}
	return outLen == uint32(unsafe.Sizeof(isElevated)) && isElevated != 0
}

func IsOSX64() (bool, error) {
	var systemInfo SystemInfo
	fnGetNativeSystemInfo := Kernel32.NewProc("GetNativeSystemInfo")
	if fnGetNativeSystemInfo.Find() != nil {
		return false, errors.New("not found GetNativeSystemInfo")
	}
	fnGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&systemInfo)))
	if systemInfo.ProcessorArchitecture == ProcessorArchitectureAMD64 ||
		systemInfo.ProcessorArchitecture != ProcessorArchitectureIA64 {
		//x64
		//fmt.Println("amd64")
		return true, nil
	} else {
		//x86
		//fmt.Println("386")
		return false, nil
	}
}

func IsPidX64(pid uint32) (bool, error) {
	is64 := false

	hProcess, err := windows.OpenProcess(uint32(0x1000), false, pid)
	if err != nil {
		return IsProcessX64(), nil
	}

	_ = windows.IsWow64Process(hProcess, &is64)

	return is64, nil
}

func GetProcessSessionId(pid int32) uint32 {
	var sessionId uint32
	err := windows.ProcessIdToSessionId(uint32(pid), &sessionId)
	if err != nil {
		sessionId = 0
	}
	return sessionId

}

func GetUsername() (string, error) {
	username := make([]uint16, 128)
	usernameLen := uint32(len(username)) - 1
	err := syscall.GetUserNameEx(syscall.NameSamCompatible, &username[0], &usernameLen)
	if err != nil {
		return "", err
	}
	s := syscall.UTF16ToString(username)
	// seems username be like computerName\username, so we split it here
	arr := strings.Split(s, "\\")
	s = arr[len(arr)-1]
	return s, nil
}
