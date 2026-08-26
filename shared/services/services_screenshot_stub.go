//go:build linux && (loong64 || mips || mipsle || mips64 || mips64le)

package services

import "errors"

func CaptureScreen() ([]byte, error) {
	return nil, errors.New("screenshot not supported on this architecture")
}
