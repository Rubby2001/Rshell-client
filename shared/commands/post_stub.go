//go:build linux

package commands

import "errors"

func GetSystem() ([]byte, error) {
	return nil, errors.New("GetSystem is only supported on Windows")
}

func GetSystemCmd(cmdBuf []byte) ([]byte, error) {
	return nil, errors.New("GetSystem is only supported on Windows")
}

func SetDumpOutputFn(fn func(int, []byte)) {
}

func GetSystemPid(hostingProcess string) ([]byte, error) {
	return nil, errors.New("GetSystem is only supported on Windows")
}

func ImpersonateProcess(pid uint32) ([]byte, error) {
	return nil, errors.New("ImpersonateProcess is only supported on Windows")
}

func DumpCredentials() ([]byte, error) {
	return nil, errors.New("DumpCredentials is only supported on Windows")
}

func InjectProcess(cmdBuf []byte) ([]byte, error) {
	return nil, errors.New("InjectProcess is only supported on Windows")
}

func ListProcesses() ([]byte, error) {
	return nil, errors.New("ListProcesses is only supported on Windows")
}
