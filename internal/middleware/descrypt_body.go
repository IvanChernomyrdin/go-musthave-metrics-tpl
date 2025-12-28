package middleware

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"

	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
)

// загружает приватный ключ RSA
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("invalid private key format")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privKey, nil
}

// расшифровывает данные RSA
func DecryptWithRSA(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data for RSA decryption")
	}
	return rsa.DecryptPKCS1v15(rand.Reader, priv, data)
}

// расшифровывает гибридные данные
func DecryptHybridAESRSA(priv *rsa.PrivateKey, payload []byte) ([]byte, error) {
	parts := bytes.SplitN(payload, []byte("|"), 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid hybrid payload format")
	}

	encAESKey, err := base64.StdEncoding.DecodeString(string(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid base64 AES key: %w", err)
	}
	aesKey, err := rsa.DecryptPKCS1v15(rand.Reader, priv, encAESKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt AES key: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(string(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid base64 nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(string(parts[2]))
	if err != nil {
		return nil, fmt.Errorf("invalid base64 ciphertext: %w", err)
	}

	if len(nonce) == 0 || len(ciphertext) == 0 {
		return nil, fmt.Errorf("empty nonce or ciphertext")
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt AES-GCM payload: %w", err)
	}

	return plaintext, nil
}

// расшифровывает тело запроса по заголовку X-Encrypted
func DecryptMiddleware(privKeyPath string) func(http.Handler) http.Handler {
	privKey, err := LoadPrivateKey(privKeyPath)
	if err != nil {
		logger.NewHTTPLogger().Sugar().Fatalf("failed to load private key: %v", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			encType := r.Header.Get("X-Encrypted")
			if encType == "" {
				next.ServeHTTP(w, r)
				return
			}

			bodyData, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "cannot read request body", http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			if len(bodyData) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			var decrypted []byte
			switch encType {
			case "rsa":
				decrypted, err = DecryptWithRSA(privKey, bodyData)
			case "hybrid":
				decrypted, err = DecryptHybridAESRSA(privKey, bodyData)
			default:
				http.Error(w, "unsupported encryption type", http.StatusBadRequest)
				return
			}

			if err != nil {
				logger.NewHTTPLogger().Sugar().Warnf("decryption failed for %s: %v", r.RequestURI, err)
				http.Error(w, "failed to decrypt payload", http.StatusBadRequest)
				return
			}

			logger.NewHTTPLogger().Sugar().Infof("successfully decrypted request body for %s using %s", r.RequestURI, encType)
			r.Body = io.NopCloser(bytes.NewReader(decrypted))
			next.ServeHTTP(w, r)
		})
	}
}
