//go:build !(linux && (loong64 || mips || mipsle || mips64 || mips64le))

package services

import (
	"bytes"
	"errors"
	"image/png"

	"github.com/kbinani/screenshot"
)

func CaptureScreen() ([]byte, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, errors.New("no displays found")
	}
	img, err := screenshot.CaptureDisplay(0)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
