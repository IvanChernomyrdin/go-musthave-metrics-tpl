// Package tests
package tests

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
)

// helper для генерации пары RSA ключей
func generateRSAKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey, string) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	pub := &priv.PublicKey

	// сохраняем приватный ключ во временный файл
	tmpFile, err := os.CreateTemp("", "priv*.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()

	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(tmpFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}); err != nil {
		t.Fatal(err)
	}

	return priv, pub, tmpFile.Name()
}

func encryptRSA(pub *rsa.PublicKey, data []byte) []byte {
	ciphertext, _ := rsa.EncryptPKCS1v15(rand.Reader, pub, data)
	return ciphertext
}

func encryptHybrid(pub *rsa.PublicKey, plaintext []byte) []byte {
	// AES ключ
	aesKey := make([]byte, 32)
	rand.Read(aesKey)

	block, _ := aes.NewCipher(aesKey)
	gcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	encAESKey, _ := rsa.EncryptPKCS1v15(rand.Reader, pub, aesKey)

	payload := []byte(base64.StdEncoding.EncodeToString(encAESKey) + "|" +
		base64.StdEncoding.EncodeToString(nonce) + "|" +
		base64.StdEncoding.EncodeToString(ciphertext))
	return payload
}

func TestDecryptMiddleware(t *testing.T) {
	_, pub, privPath := generateRSAKeys(t)
	defer os.Remove(privPath)

	// тестовый handler
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		body, _ := io.ReadAll(r.Body)
		expected := []byte("secret message")
		if !bytes.Equal(body, expected) {
			t.Errorf("body mismatch, got %s", string(body))
		}
	})

	// middleware
	mw := middleware.DecryptMiddleware(privPath)
	testHandler := mw(handler)

	tests := []struct {
		name      string
		body      []byte
		header    string
		encryptFn func() []byte
	}{
		{
			name:   "RSA decryption",
			header: "rsa",
			encryptFn: func() []byte {
				return encryptRSA(pub, []byte("secret message"))
			},
		},
		{
			name:   "Hybrid AES-RSA decryption",
			header: "hybrid",
			encryptFn: func() []byte {
				return encryptHybrid(pub, []byte("secret message"))
			},
		},
		{
			name:      "no encryption",
			header:    "",
			encryptFn: func() []byte { return []byte("secret message") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(tt.encryptFn()))
			if tt.header != "" {
				req.Header.Set("X-Encrypted", tt.header)
			}
			w := httptest.NewRecorder()

			handlerCalled = false
			testHandler.ServeHTTP(w, req)

			if !handlerCalled {
				t.Errorf("handler was not called")
			}

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("unexpected status code %d", resp.StatusCode)
			}
		})
	}
}
