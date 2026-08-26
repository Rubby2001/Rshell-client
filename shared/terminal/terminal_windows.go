//go:build windows
// +build windows

package terminal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/ActiveState/termtest/conpty"
	"github.com/NHAS/reverse_ssh/pkg/winpty"
	"golang.org/x/sys/windows"
)

const (
	cpACP  = 0
	cpUTF8 = 65001
)

var (
	modkernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procWideCharToMultiByte = modkernel32.NewProc("WideCharToMultiByte")
)

// Client 终端客户端接口
type Client interface {
	ExecuteCommand(cmd []byte) ([]byte, error)
	StartInteractive(width, height int) (chan<- []byte, <-chan []byte, error)
	Resize(width, height int) error
	Close() error
}

// terminalClient Windows实现
type terminalClient struct {
	cmd       *exec.Cmd
	pty       interface{} // 可以是winpty.WinPTY或conpty.ConPty
	running   bool
	mutex     sync.Mutex
	useConPty bool // 标记是否使用ConPty
}

// NewClient 创建新的终端客户端
func NewClient() Client {
	// 检查Windows版本，决定使用ConPty还是WinPty
	vsn := windows.RtlGetVersion()
	useConPty := vsn.MajorVersion >= 10 && vsn.BuildNumber >= 17763

	return &terminalClient{
		useConPty: useConPty,
	}
}

// ExecuteCommand 执行单个命令
func (tc *terminalClient) ExecuteCommand(cmd []byte) ([]byte, error) {
	// 对于单次命令执行，我们仍然使用原来的方法
	c := exec.Command("cmd.exe", "/c", "chcp 65001 >nul && "+string(cmd))

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	if err != nil {
		output := stdout.Bytes()
		// 尝试将 GBK 转换为 UTF-8
		if utf8.Valid(output) {
			return append(output, stderr.Bytes()...), err
		}
		utf8Output, _ := GBKToUTF8(output)
		return append(utf8Output, stderr.Bytes()...), err
	}

	output := stdout.Bytes()
	// 尝试将 GBK 转换为 UTF-8
	if utf8.Valid(output) {
		return output, nil
	}
	utf8Output, _ := GBKToUTF8(output)
	return utf8Output, nil
}

// StartInteractive 启动交互式终端
func (tc *terminalClient) StartInteractive(width, height int) (chan<- []byte, <-chan []byte, error) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if tc.running {
		return nil, nil, fmt.Errorf("terminal already running")
	}

	// 创建通道
	inputChan := make(chan []byte, 100)
	outputChan := make(chan []byte, 100)

	// 启动PTY
	var err error
	if tc.useConPty {
		err = tc.startConPty(width, height)
	} else {
		err = tc.startWinPty(width, height)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("start PTY: %w", err)
	}

	tc.running = true

	// 启动协程处理输入输出
	go tc.readOutput(outputChan)
	go tc.handleInput(inputChan)

	return inputChan, outputChan, nil
}

// startConPty 启动ConPty（Windows 10 1809+）
func (tc *terminalClient) startConPty(width, height int) error {
	// 首先查找cmd.exe的完整路径
	cmdPath, err := exec.LookPath("cmd.exe")
	if err != nil {
		return fmt.Errorf("find cmd.exe: %w", err)
	}

	// 创建ConPty
	cpty, err := conpty.New(int16(width), int16(height))
	if err != nil {
		return fmt.Errorf("create ConPty: %w", err)
	}

	// 启动命令 - 使用完整的路径和参数
	pid, _, err := cpty.Spawn(
		cmdPath,      // 使用完整的路径
		[]string{""}, // 注意：这里添加了 cls
		&syscall.ProcAttr{
			Env: os.Environ(), // 环境变量
		},
	)
	if err != nil {
		cpty.Close()
		return fmt.Errorf("spawn process: %w", err)
	}

	// 查找进程
	process, err := os.FindProcess(pid)
	if err != nil {
		cpty.Close()
		return fmt.Errorf("find process: %w", err)
	}

	tc.cmd = &exec.Cmd{
		Process: process,
		Path:    cmdPath,
		Args:    []string{""},
	}
	tc.pty = cpty

	// 添加延迟，等待终端初始化完成
	time.Sleep(50 * time.Millisecond)

	// 发送复位序列确保光标在正确位置
	resetSeq := []byte("\x1b[2J\x1b[H")
	cpty.InPipe().Write(resetSeq)

	return nil
}

// startWinPty 启动WinPty（旧版Windows）
func (tc *terminalClient) startWinPty(width, height int) error {
	// 首先查找cmd.exe的完整路径
	cmdPath, err := exec.LookPath("cmd.exe")
	if err != nil {
		return fmt.Errorf("find cmd.exe: %w", err)
	}

	// 创建完整的命令字符串
	fullCommand := fmt.Sprintf(`"%s" /k chcp 65001 >nul && cls`, cmdPath)

	// 创建WinPty
	options := winpty.Options{
		Command:     fullCommand,
		Dir:         "",           // 当前工作目录
		Env:         os.Environ(), // 环境变量
		InitialCols: uint32(width),
		InitialRows: uint32(height),
		Flags:       0, // 默认标志
	}

	winptyObj, err := winpty.OpenWithOptions(options)
	if err != nil {
		return fmt.Errorf("create WinPty: %w", err)
	}

	// 创建exec.Cmd以便监控进程
	cmd := exec.Command(cmdPath, "")

	tc.cmd = cmd
	tc.pty = winptyObj

	// 添加延迟，等待终端初始化完成
	time.Sleep(50 * time.Millisecond)

	// 发送复位序列确保光标在正确位置
	resetSeq := []byte("\x1b[2J\x1b[H")
	winptyObj.Write(resetSeq)

	return nil
}

// Resize 调整终端大小
func (tc *terminalClient) Resize(width, height int) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if !tc.running {
		return fmt.Errorf("terminal not running")
	}

	switch pty := tc.pty.(type) {
	case *conpty.ConPty:
		return pty.Resize(uint16(width), uint16(height))
	case *winpty.WinPTY:
		pty.SetSize(uint32(width), uint32(height))
		return nil
	default:
		return fmt.Errorf("unknown PTY type")
	}
}

// handleInput 处理输入
func (tc *terminalClient) handleInput(inputChan <-chan []byte) {
	for cmd := range inputChan {
		tc.mutex.Lock()
		if !tc.running {
			tc.mutex.Unlock()
			return
		}

		// 处理输入数据
		processedCmd := tc.processWindowsInput(cmd)

		// 发送到PTY
		var err error
		switch pty := tc.pty.(type) {
		case *conpty.ConPty:
			_, err = pty.InPipe().Write(processedCmd)
		case *winpty.WinPTY:
			_, err = pty.Write(processedCmd)
		}
		tc.mutex.Unlock()

		if err != nil {
			continue
		}
	}
}

// processWindowsInput 处理Windows输入的特殊字符
func (tc *terminalClient) processWindowsInput(cmd []byte) []byte {
	if len(cmd) == 0 {
		return cmd
	}

	// 确保输入是UTF-8
	utf8Cmd := cmd
	if !utf8.Valid(cmd) {
		utf8Cmd = ensureUTF8(cmd)
	}

	// 处理换行符：将\n转换为\r\n
	result := make([]byte, 0, len(utf8Cmd)+10)
	for i, b := range utf8Cmd {
		if b == '\n' && (i == 0 || utf8Cmd[i-1] != '\r') {
			result = append(result, '\r', '\n')
		} else {
			result = append(result, b)
		}
	}
	return result
}

// readOutput 读取输出
func (tc *terminalClient) readOutput(outputChan chan<- []byte) {
	buf := make([]byte, 8192)

	for {
		tc.mutex.Lock()
		if !tc.running {
			tc.mutex.Unlock()
			close(outputChan)
			return
		}

		var reader io.Reader
		switch pty := tc.pty.(type) {
		case *conpty.ConPty:
			reader = pty.OutPipe()
		case *winpty.WinPTY:
			reader = pty
		default:
			tc.mutex.Unlock()
			continue
		}
		tc.mutex.Unlock()

		n, err := reader.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			select {
			case outputChan <- data:
			default:
				time.Sleep(10 * time.Millisecond)
				select {
				case outputChan <- data:
				case <-time.After(50 * time.Millisecond):
				}
			}
		}

		if err != nil {
			close(outputChan)
			return
		}
	}
}

// Close 关闭终端
func (tc *terminalClient) Close() error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if tc.running {
		tc.running = false

		switch pty := tc.pty.(type) {
		case *conpty.ConPty:
			pty.Close()
		case *winpty.WinPTY:
			pty.Close()
		}

		if tc.cmd != nil && tc.cmd.Process != nil {
			tc.cmd.Process.Kill()
		}
	}

	return nil
}

func gbkToUTF16(b []byte) ([]uint16, error) {
	if len(b) == 0 {
		return nil, nil
	}
	n, err := windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), nil, 0)
	if n == 0 {
		return nil, err
	}
	utf16 := make([]uint16, n)
	_, err = windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), &utf16[0], n)
	return utf16, err
}

// GBKToUTF8 将GBK编码转换为UTF-8
func GBKToUTF8(gbk []byte) ([]byte, error) {
	if utf8.Valid(gbk) {
		return gbk, nil
	}
	utf16, err := gbkToUTF16(gbk)
	if err != nil {
		return gbk, nil
	}
	return utf16ToUTF8(utf16), nil
}

// UTF8ToGBK 将UTF-8编码转换为GBK
func UTF8ToGBK(utf8Data []byte) ([]byte, error) {
	if !utf8.Valid(utf8Data) {
		return utf8Data, nil
	}
	utf16 := utf8ToUTF16(utf8Data)
	return utf16ToGBK(utf16), nil
}

func utf16ToUTF8(utf16 []uint16) []byte {
	var buf []byte
	for _, r := range utf16 {
		if r < 0x80 {
			buf = append(buf, byte(r))
		} else if r < 0x800 {
			buf = append(buf, byte(0xC0|(r>>6)), byte(0x80|(r&0x3F)))
		} else {
			buf = append(buf, byte(0xE0|(r>>12)), byte(0x80|((r>>6)&0x3F)), byte(0x80|(r&0x3F)))
		}
	}
	return buf
}

func utf8ToUTF16(utf8Data []byte) []uint16 {
	runes := []rune(string(utf8Data))
	utf16 := make([]uint16, 0, len(runes))
	for _, r := range runes {
		if r < 0x10000 {
			utf16 = append(utf16, uint16(r))
		} else {
			r -= 0x10000
			utf16 = append(utf16, uint16(0xD800+((r>>10)&0x3FF)), uint16(0xDC00+(r&0x3FF)))
		}
	}
	return utf16
}

func utf16ToGBK(utf16 []uint16) []byte {
	if len(utf16) == 0 {
		return nil
	}
	ret, _, _ := procWideCharToMultiByte.Call(
		cpACP, 0,
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)),
		0, 0, 0, 0,
	)
	if ret == 0 {
		return nil
	}
	out := make([]byte, ret)
	ret, _, _ = procWideCharToMultiByte.Call(
		cpACP, 0,
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)),
		uintptr(unsafe.Pointer(&out[0])),
		ret, 0, 0,
	)
	if ret == 0 {
		return nil
	}
	return out
}

// ensureUTF8 确保字节数组是UTF-8编码
func ensureUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}

	// 尝试从GBK转换
	utf8Data, err := GBKToUTF8(data)
	if err == nil {
		return utf8Data
	}

	// 如果转换失败，返回原始数据
	return data
}
