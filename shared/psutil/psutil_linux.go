//go:build linux

package psutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func processes() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var result []ProcessInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		fields := strings.Fields(string(stat))
		if len(fields) < 4 {
			continue
		}
		name := strings.Trim(fields[1], "()")
		ppid, _ := strconv.Atoi(fields[3])
		user, _ := username(int32(pid))
		result = append(result, ProcessInfo{
			Pid:      int32(pid),
			PPid:     int32(ppid),
			Name:     name,
			Username: user,
		})
	}
	return result, nil
}

func username(pid int32) (string, error) {
	status, err := os.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "status"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid := fields[1]
				return uidToUsername(uid), nil
			}
		}
	}
	return "", fmt.Errorf("uid not found")
}

func uidToUsername(uid string) string {
	content, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return uid
	}
	for _, line := range strings.Split(string(content), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[2] == uid {
			return parts[0]
		}
	}
	return uid
}
