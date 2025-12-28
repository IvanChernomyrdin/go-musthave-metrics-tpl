package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// загружает публичный ключ RSA
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("invalid public key format")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// Шифрует данные публичным ключом
func EncryptWithRSA(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, pub, data)
}

// Шифрование для больших батчей
func EncryptHybridAESRSA(pubKey *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	aesKey := make([]byte, 32) // AES-256
	if _, err := rand.Read(aesKey); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	encAESKey, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, aesKey)
	if err != nil {
		return nil, err
	}

	payload := fmt.Sprintf("%s|%s|%s",
		base64.StdEncoding.EncodeToString(encAESKey),
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ciphertext),
	)

	return []byte(payload), nil
}
