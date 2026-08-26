//go:build !windows

package utils

func codepageToUTF8(b []byte) ([]byte, error) {
	return b, nil
}
