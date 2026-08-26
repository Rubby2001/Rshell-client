//go:build !windows
// +build !windows

package terminal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/creack/pty"
)

// Client 终端客户端接口
type Client interface {
	ExecuteCommand(cmd []byte) ([]byte, error)
	StartInteractive(width, height int) (chan<- []byte, <-chan []byte, error)
	Close() error
}

// terminalClient Unix实现
type terminalClient struct {
	cmd     *exec.Cmd
	pty     *os.File
	running bool
	mutex   sync.Mutex
}

// NewClient 创建新的终端客户端
func NewClient() Client {
	return &terminalClient{}
}

// ExecuteCommand 执行单个命令
func (tc *terminalClient) ExecuteCommand(cmd []byte) ([]byte, error) {
	var c *exec.Cmd
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		c = exec.Command("sh", "-c", string(cmd))
	} else {
		c = exec.Command("bash", "-c", string(cmd))
	}

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	if err != nil {
		return append(stdout.Bytes(), stderr.Bytes()...), err
	}

	return stdout.Bytes(), nil
}

// findAvailableShell 查找可用的 shell
func findAvailableShell() (string, error) {
	// 定义 shell 的优先级列表
	possibleShells := []string{
		"/bin/bash",
		"/usr/bin/bash",
		"/bin/zsh",
		"/usr/bin/zsh",
		"/bin/sh",
		"/usr/bin/sh",
	}

	// 根据操作系统调整优先级
	if runtime.GOOS == "darwin" {
		// macOS 默认使用 zsh (从 Catalina 开始)
		possibleShells = []string{
			"/bin/zsh",
			"/bin/bash",
			"/usr/local/bin/bash",
			"/bin/sh",
		}
	}

	// 检查每个 shell 是否存在
	for _, shell := range possibleShells {
		if _, err := os.Stat(shell); err == nil {
			return shell, nil
		}
	}

	// 回退到使用 which/whereis 命令查找
	fallbackShells := []string{"bash", "zsh", "sh"}
	for _, shell := range fallbackShells {
		if path, err := exec.LookPath(shell); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no available shell found. Tried: %v", possibleShells)
}

// StartInteractive 启动交互式终端
func (tc *terminalClient) StartInteractive(width, height int) (chan<- []byte, <-chan []byte, error) {
	// 尝试找到可用的 shell
	shell, err := findAvailableShell()
	if err != nil {
		return nil, nil, fmt.Errorf("find available shell: %w", err)
	}

	cmd := exec.Command(shell, "-i") // -i 表示交互模式

	// 创建PTY
	var ptyErr error
	tc.pty, ptyErr = pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})
	if ptyErr != nil {
		return nil, nil, fmt.Errorf("start PTY: %w", ptyErr)
	}

	tc.cmd = cmd
	tc.running = true

	// 创建通道
	inputChan := make(chan []byte, 100)
	outputChan := make(chan []byte, 100)

	// 使用WaitGroup确保所有goroutine正确退出
	var wg sync.WaitGroup
	wg.Add(2) // readOutput 和 monitorProcess

	// 启动协程
	go tc.readOutput(outputChan, &wg)
	go tc.handleInput(inputChan)
	go tc.monitorProcess(outputChan, &wg)

	// 启动一个goroutine等待所有输出goroutine完成，然后关闭outputChan
	go func() {
		wg.Wait()
		close(outputChan)
	}()

	return inputChan, outputChan, nil
}

// handleInput 处理输入
func (tc *terminalClient) handleInput(inputChan <-chan []byte) {
	for cmd := range inputChan {
		tc.mutex.Lock()
		if !tc.running || tc.pty == nil {
			tc.mutex.Unlock()
			return
		}

		// 发送命令到PTY
		_, err := tc.pty.Write(cmd)
		tc.mutex.Unlock()

		if err != nil {
			// 写入失败，继续处理但不退出
			continue
		}
	}
}

// readOutput 读取输出
func (tc *terminalClient) readOutput(outputChan chan<- []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	buf := make([]byte, 4096)

	for {
		tc.mutex.Lock()
		if !tc.running || tc.pty == nil {
			tc.mutex.Unlock()
			return
		}
		tc.mutex.Unlock()

		n, err := tc.pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				// 发送错误信息
				tc.mutex.Lock()
				if tc.running {
					outputChan <- []byte(fmt.Sprintf("Read error: %v\n", err))
				}
				tc.mutex.Unlock()
			}
			return
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			tc.mutex.Lock()
			if tc.running {
				outputChan <- data
			}
			tc.mutex.Unlock()
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// monitorProcess 监控进程状态
func (tc *terminalClient) monitorProcess(outputChan chan<- []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	err := tc.cmd.Wait()
	tc.mutex.Lock()
	tc.running = false
	tc.mutex.Unlock()

	// 发送退出消息（如果outputChan还没关闭）
	tc.mutex.Lock()
	if err != nil {
		select {
		case outputChan <- []byte(fmt.Sprintf("Process exited with error: %v\n", err)):
		default:
			// 通道已满或关闭，不阻塞
		}
	} else {
		select {
		case outputChan <- []byte("Process exited normally\n"):
		default:
			// 通道已满或关闭，不阻塞
		}
	}
	tc.mutex.Unlock()
}

// Close 关闭终端
func (tc *terminalClient) Close() error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if tc.running {
		tc.running = false
		if tc.pty != nil {
			tc.pty.Close()
		}
		if tc.cmd != nil && tc.cmd.Process != nil {
			tc.cmd.Process.Kill()
		}
	}

	return nil
}
