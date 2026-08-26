package services

import (
	"rshell-client/shared/link"
	"rshell-client/shared/terminal"
	"rshell-client/shared/utils"
	"bytes"
	"errors"
	"sync"
	"time"
)

// Session 表示一个终端会话
type Session struct {
	ID           string
	Client       terminal.Client
	InputChan    chan<- []byte
	OutputChan   <-chan []byte
	mu           sync.RWMutex
	lastOutput   []byte
	outputBuffer []byte
	closed       bool
}

// SessionManager 管理所有会话
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var (
	sessionManager *SessionManager
	once           sync.Once
)

// GetSessionManager 获取单例会话管理器
func GetSessionManager() *SessionManager {
	once.Do(func() {
		sessionManager = &SessionManager{
			sessions: make(map[string]*Session),
		}
	})
	return sessionManager
}

// NewSession 创建新会话
func (sm *SessionManager) NewSession(sessionID string, width, height int) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查会话是否已存在
	if _, exists := sm.sessions[sessionID]; exists {
		return nil, errors.New("session already exists")
	}

	// 创建终端客户端
	client := terminal.NewClient()
	inputChan, outputChan, err := client.StartInteractive(width, height)
	if err != nil {
		return nil, err
	}

	// 创建会话
	session := &Session{
		ID:         sessionID,
		Client:     client,
		InputChan:  inputChan,
		OutputChan: outputChan,
	}

	// 启动输出收集协程
	go session.collectOutput()

	// 存储会话
	sm.sessions[sessionID] = session

	return session, nil
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(sessionID string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}

	return session, nil
}

// CloseSession 关闭会话
func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	// 关闭会话
	session.Close()

	// 从管理器中移除
	delete(sm.sessions, sessionID)

	return nil
}

// collectOutput 收集会话输出
//
//	func (s *Session) collectOutput() {
//		for output := range s.OutputChan {
//			s.mu.Lock()
//			// 存储最后输出（用于非阻塞读取）
//			s.lastOutput = output
//			// 追加到缓冲区
//			s.outputBuffer = append(s.outputBuffer, output...)
//			s.mu.Unlock()
//
//			buf := bytes.NewBuffer(nil)
//			sessionIDBytes := []byte(s.ID)
//			sessionIDLen := len(sessionIDBytes)
//			buf.Write(utils.WriteInt(sessionIDLen)) // 4字节长度
//			buf.Write(sessionIDBytes)               // sessionID
//			buf.Write(output)
//			link.ReportData(WriteInteractieShell, buf.Bytes())
//		}
//
//		s.mu.Lock()
//		s.closed = true
//		s.mu.Unlock()
//	}
func (s *Session) collectOutput() {
	for output := range s.OutputChan {
		s.mu.Lock()
		s.lastOutput = output
		s.outputBuffer = append(s.outputBuffer, output...)
		s.mu.Unlock()

		buf := bytes.NewBuffer(nil)
		sessionIDBytes := []byte(s.ID)
		sessionIDLen := len(sessionIDBytes)
		buf.Write(utils.WriteInt(sessionIDLen))
		buf.Write(sessionIDBytes)
		buf.Write(output)
		link.ReportData(WriteInteractieShell, buf.Bytes())
	}
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}
func shouldEcho(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// ESC 开头（方向键 / 功能键）
	if data[0] == 0x1b {
		return false
	}

	for _, b := range data {
		// 控制字符
		if b < 0x20 || b == 0x7f {
			return false
		}
	}

	return true
}

// Write 向会话写入命令
// Write 向会话写入命令（修复通道阻塞问题）
func (s *Session) Write(command []byte) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return errors.New("session closed")
	}
	s.mu.RUnlock()

	// 使用select避免通道满时永久阻塞，同时保留阻塞特性（可添加超时）
	select {
	case s.InputChan <- command: // 通道有缓冲空间，正常发送
		return nil
	case <-time.After(3 * time.Second): // 3秒超时保护，避免无限阻塞
		return errors.New("write timeout: input channel is full")
	}
}

// WaitForOutput 等待输出
// func (s *Session) WaitForOutput(timeoutMs int) ([]byte, error) {
// 	// 简单实现：等待一段时间后返回缓冲区内容
// 	// 在实际应用中，你可能需要更复杂的输出检测逻辑

// 	// 等待一小段时间让命令执行
// 	// time.Sleep(time.Duration(timeoutMs/10) * time.Millisecond)

// 	s.mu.RLock()
// 	defer s.mu.RUnlock()

// 	if len(s.outputBuffer) > 0 {
// 		result := make([]byte, len(s.outputBuffer))
// 		copy(result, s.outputBuffer)
// 		return result, nil
// 	}

// 	return []byte{}, nil
// }

// Close 关闭会话
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		close(s.InputChan)
		s.Client.Close()
	}
}

// // GetLastOutput 获取最后输出
// func (s *Session) GetLastOutput() []byte {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()

// 	return s.lastOutput
// }

// // ReadOutput 读取当前输出缓冲区
// func (s *Session) ReadOutput() []byte {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()

// 	if len(s.outputBuffer) > 0 {
// 		result := make([]byte, len(s.outputBuffer))
// 		copy(result, s.outputBuffer)
// 		return result
// 	}

// 	return nil
// }
