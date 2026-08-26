//go:build darwin

package psutil

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func processes() ([]ProcessInfo, error) {
	out, err := exec.Command("ps", "-eo", "pid,ppid,comm,user").Output()
	if err != nil {
		return nil, fmt.Errorf("ps command failed: %v", err)
	}
	var result []ProcessInfo
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		result = append(result, ProcessInfo{
			Pid:      int32(pid),
			PPid:     int32(ppid),
			Name:     fields[2],
			Username: fields[3],
		})
	}
	return result, nil
}

func username(pid int32) (string, error) {
	out, err := exec.Command("ps", "-o", "user=", "-p", strconv.Itoa(int(pid))).Output()
	if err != nil {
		return "", fmt.Errorf("ps command failed: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}
