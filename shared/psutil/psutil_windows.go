//go:build windows

package psutil

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

var (
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	modadvapi32              = windows.NewLazySystemDLL("advapi32.dll")
	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First       = modkernel32.NewProc("Process32FirstW")
	procProcess32Next        = modkernel32.NewProc("Process32NextW")
	procOpenProcess          = modkernel32.NewProc("OpenProcess")
	procCloseHandle          = modkernel32.NewProc("CloseHandle")
	procOpenProcessToken     = modadvapi32.NewProc("OpenProcessToken")
	procGetTokenInformation  = modadvapi32.NewProc("GetTokenInformation")
	procLookupAccountSidW    = modadvapi32.NewProc("LookupAccountSidW")
)

const (
	TH32CS_SNAPPROCESS      = 0x00000002
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ         = 0x0010
	TOKEN_QUERY             = 0x0008
)

type PROCESSENTRY32W struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}

type SIDAndAttributes struct {
	Sid        *windows.SID
	Attributes uint32
}

type TOKEN_USER struct {
	User SIDAndAttributes
}

func processes() ([]ProcessInfo, error) {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == uintptr(0xFFFFFFFF) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snapshot)

	var pe PROCESSENTRY32W
	pe.DwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32First failed")
	}

	var result []ProcessInfo
	for {
		name := windows.UTF16ToString(pe.SzExeFile[:])
		user, _ := username(int32(pe.Th32ProcessID))
		result = append(result, ProcessInfo{
			Pid:      int32(pe.Th32ProcessID),
			PPid:     int32(pe.Th32ParentProcessID),
			Name:     name,
			Username: user,
		})

		ret, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
		if ret == 0 {
			break
		}
	}
	return result, nil
}

func username(pid int32) (string, error) {
	handle, _, _ := procOpenProcess.Call(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, 0, uintptr(pid))
	if handle == 0 {
		return "", fmt.Errorf("OpenProcess failed")
	}
	defer procCloseHandle.Call(handle)

	var tokenHandle windows.Handle
	ret, _, _ := procOpenProcessToken.Call(handle, TOKEN_QUERY, uintptr(unsafe.Pointer(&tokenHandle)))
	if ret == 0 {
		return "", fmt.Errorf("OpenProcessToken failed")
	}
	defer procCloseHandle.Call(uintptr(tokenHandle))

	var returnLen uint32
	windows.GetTokenInformation(windows.Token(tokenHandle), windows.TokenUser, nil, 0, &returnLen)

	tokenBuf := make([]byte, returnLen)
	err := windows.GetTokenInformation(windows.Token(tokenHandle), windows.TokenUser, &tokenBuf[0], uint32(len(tokenBuf)), &returnLen)
	if err != nil {
		return "", err
	}

	tokenUser := *(*TOKEN_USER)(unsafe.Pointer(&tokenBuf[0]))

	var nameLen uint32 = 256
	var domainLen uint32 = 256
	nameBuf := make([]uint16, nameLen)
	domainBuf := make([]uint16, domainLen)
	var sidUse uint32

	ret, _, _ = procLookupAccountSidW.Call(
		0,
		uintptr(unsafe.Pointer(tokenUser.User.Sid)),
		uintptr(unsafe.Pointer(&nameBuf[0])),
		uintptr(unsafe.Pointer(&nameLen)),
		uintptr(unsafe.Pointer(&domainBuf[0])),
		uintptr(unsafe.Pointer(&domainLen)),
		uintptr(unsafe.Pointer(&sidUse)),
	)
	if ret == 0 {
		return "", fmt.Errorf("LookupAccountSid failed")
	}

	username := windows.UTF16ToString(nameBuf[:nameLen])
	return username, nil
}
