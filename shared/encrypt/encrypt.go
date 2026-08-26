package encrypt

import (
	"rshell-client/shared/config"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	r "math/rand"

	"golang.org/x/crypto/curve25519"
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

func EncryptNormal(data []byte) ([]byte, error) {
	key := generateKey()
	encryptedData, _ := encryptAES([]byte(hex.EncodeToString(data)), key)
	// key与加密结果放到一起
	keyAndData := append(key, encryptedData...)
	return keyAndData, nil
}
func RandomInt(min, max int) int {
	return min + r.Intn(max-min)
}

func DecryptNormal(data []byte) ([]byte, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("data is too short")
	}
	key := data[:32]
	encryptedData := data[32:]
	decryptedData, _ := decryptAES(encryptedData, key)
	plainData, _ := hex.DecodeString(string(decryptedData))
	return plainData, nil
}
func Encrypt(data []byte) ([]byte, error) {
	var pubKey [32]byte
	copy(pubKey[:], config.ServerPublicKeyBytes[:32])
	key, _ := ComputeSharedSecret(Key.Private, pubKey)
	encryptedData, _ := encryptAES([]byte(hex.EncodeToString(data)), key[:])
	return encryptedData, nil
}

func Decrypt(data []byte) ([]byte, error) {
	var pubKey [32]byte
	copy(pubKey[:], config.ServerPublicKeyBytes[:32])
	key, _ := ComputeSharedSecret(Key.Private, pubKey)
	decryptedData, _ := decryptAES(data, key[:])
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

type KeyPair struct {
	Public  [32]byte
	Private [32]byte
}

var Key KeyPair

func GenerateKeyPair() {
	var publicKey, privateKey [32]byte

	// 生成私钥（需要随机数）
	if _, err := rand.Read(privateKey[:]); err != nil {
		return
	}

	// 计算公钥
	curve25519.ScalarBaseMult(&publicKey, &privateKey)
	Key = KeyPair{
		Public:  publicKey,
		Private: privateKey,
	}
	return
}

func BytesToMD5(s []byte) string {
	h := md5.New()
	h.Write(s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func ComputeSharedSecret(privateKey, peerPublicKey [32]byte) ([32]byte, error) {
	sharedSecret, err := curve25519.X25519(privateKey[:], peerPublicKey[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	var result [32]byte
	copy(result[:], sharedSecret)

	return result, nil
}
