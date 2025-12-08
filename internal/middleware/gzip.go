package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// GzipDecompression middleware распаковывает входящие gzip данные
func GzipDecompression(httpGzip http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//по тз проверяем content-encoding gzip
		contentEncoding := r.Header.Get("Content-Encoding")

		if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
			//если тело пустое выходим
			if r.ContentLength == 0 || r.Body == http.NoBody {
				r.Header.Del("Content-Encoding")
				httpGzip.ServeHTTP(w, r)
				return
			}

			//создания reader для распаковки
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			//новое тело для распакованных данных
			decompressionbody, err := io.ReadAll(gz)
			if err != nil {
				http.Error(w, "Failed to decompression gz", http.StatusBadRequest)
				return
			}
			//заменяем тело запроса
			r.Body = io.NopCloser(bytes.NewReader(decompressionbody))
			//удаляем заголовок сжатия
			r.Header.Del("Content-Encoding")
			//обновляем len body content
			r.ContentLength = int64(len(decompressionbody))
		}
		httpGzip.ServeHTTP(w, r)
	})
}

type GzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w GzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w GzipResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

func GzipCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем поддержку gzip клиентом
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")

		gzrw := GzipResponseWriter{
			Writer:         gz,
			ResponseWriter: w,
		}

		next.ServeHTTP(gzrw, r)
	})
}
