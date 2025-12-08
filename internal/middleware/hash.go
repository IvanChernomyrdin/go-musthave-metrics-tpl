package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
)

type HashMiddleware struct {
	HashKey string
}

func NewHashMiddleware(HashKey string) *HashMiddleware {
	return &HashMiddleware{HashKey: HashKey}
}

// проверяем входящие запросы на хэш
func (h *HashMiddleware) CheckHash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.HashKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPut || r.Method == http.MethodPost {
			//приходящий sha256
			incomingHash := r.Header.Get("HashSHA256")
			if incomingHash != "" {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Cannot read body", http.StatusBadRequest)
					return
				}
				r.Body = io.NopCloser(bytes.NewBuffer(body))
				//используем тело для получаения sha256
				computedHash := h.ComputeHash(body)
				if !hmac.Equal([]byte(incomingHash), []byte(computedHash)) {
					// http.Error(w, "Invalid hash sum", http.StatusBadRequest)
					// return
					log.Printf("Хэши не сходятся: incoming=%s, computed=%s", incomingHash, computedHash)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *HashMiddleware) ComputeHash(body []byte) string {
	hmacHash := hmac.New(sha256.New, []byte(h.HashKey))
	hmacHash.Write(body)
	return hex.EncodeToString(hmacHash.Sum(nil))
}

type AddResponseWriter struct {
	http.ResponseWriter
	Body   []byte
	Status int
}

// добавляем хэш на отправку
func (h *HashMiddleware) AddHash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.HashKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		//перехватываем
		addRes := &AddResponseWriter{ResponseWriter: w}
		next.ServeHTTP(addRes, r)

		//вычисляем
		if len(addRes.Body) > 0 {
			hash := h.ComputeHash(addRes.Body)
			addRes.ResponseWriter.Header().Set("HashSHA256", hash)
		}
	})
}

func (w *AddResponseWriter) Write(b []byte) (int, error) {
	w.Body = append(w.Body, b...)
	return w.ResponseWriter.Write(b)
}

func (w *AddResponseWriter) WriteHeader(statusCode int) {
	w.Status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
