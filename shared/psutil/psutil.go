package psutil

type ProcessInfo struct {
	Pid      int32
	PPid     int32
	Name     string
	Username string
}

func Processes() ([]ProcessInfo, error) {
	return processes()
}

func Username(pid int32) (string, error) {
	return username(pid)
}
