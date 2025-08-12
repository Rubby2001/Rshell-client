package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	r "math/rand"
	"time"
)

// 密钥生成
func generateKey() []byte {
	key := make([]byte, 32) // 256 bits key
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}

// 加密函数
func encryptAES(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

// 解密函数
func decryptAES(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext is too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

func Encrypt(data []byte) ([]byte, error) {
	key := generateKey()
	encryptedData, _ := encryptAES([]byte(hex.EncodeToString(data)), key)
	// key与加密结果放到一起
	keyAndData := append(key, encryptedData...)
	return keyAndData, nil
}
func RandomInt(min, max int) int {
	r.Seed(time.Now().UnixNano())
	return min + r.Intn(max-min)
}

func Decrypt(data []byte) ([]byte, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("data is too short")
	}
	key := data[:32]
	encryptedData := data[32:]
	decryptedData, _ := decryptAES(encryptedData, key)
	plainData, _ := hex.DecodeString(string(decryptedData))
	return plainData, nil
}

// EncodeBase64 将 []byte 编码为 Base64 并返回 []byte
func EncodeBase64(data []byte) ([]byte, error) {
	encodedString := base64.StdEncoding.EncodeToString(data)
	return []byte(encodedString), nil
}

// DecodeBase64 将 Base64 编码的 []byte 解码回原始的 []byte
func DecodeBase64(encodedData []byte) ([]byte, error) {
	decodedData, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		return nil, err
	}
	return decodedData, nil
}
func BytesToMD5(s []byte) string {
	// 创建一个新的 MD5 哈希对象
	hasher := md5.New()

	// 将字符串写入哈希对象
	hasher.Write(s)

	// 获取哈希值的字节切片
	md5Bytes := hasher.Sum(nil)

	// 将字节切片转换为十六进制字符串
	return fmt.Sprintf("%x", md5Bytes)
}
