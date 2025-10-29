package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

type HashMiddleware struct {
	hashKey string
}

func NewHashMiddleware(hashKey string) *HashMiddleware {
	return &HashMiddleware{hashKey: hashKey}
}

// проверяем входящие запросы на хэш
func (h *HashMiddleware) CheckHash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.hashKey == "" {
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
				computerHash := h.computeHash(body)
				if !hmac.Equal([]byte(incomingHash), []byte(computerHash)) {
					http.Error(w, "Invalid hash sum", http.StatusBadRequest)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *HashMiddleware) computeHash(body []byte) string {
	hmacHash := hmac.New(sha256.New, []byte(h.hashKey))
	hmacHash.Write(body)
	return hex.EncodeToString(hmacHash.Sum(nil))
}

type addResponseWriter struct {
	http.ResponseWriter
	body   []byte
	status int
}

// добавляем хэш на отправку
func (h *HashMiddleware) AddHash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.hashKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		//перехватываем
		addRes := &addResponseWriter{ResponseWriter: w}
		next.ServeHTTP(w, r)

		//вычисляем
		if len(addRes.body) > 0 {
			hash := h.computeHash(addRes.body)
			addRes.ResponseWriter.Header().Set("HashSHA256", hash)
		}
	})
}

func (w *addResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

func (w *addResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
