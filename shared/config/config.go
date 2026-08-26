package config

import (
	"encoding/base64"
	"strings"
	"time"
)

var (
	pass                    = "PASSAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ExecuteKey              = strings.ReplaceAll(pass, " ", "")
	serverPublicKey         = "ServerPublicKeyAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ServerPublicKey         = strings.ReplaceAll(serverPublicKey, " ", "")
	ServerPublicKeyBytes, _ = base64.StdEncoding.DecodeString(ServerPublicKey)
	WaitTime                = 5000 * time.Millisecond
)
