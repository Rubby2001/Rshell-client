package config

import (
	"strings"
	"time"
)

var (
	pass       = "PASSAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ExecuteKey = strings.ReplaceAll(pass, " ", "")
	WaitTime   = 5000 * time.Millisecond
)
