package middleware

import (
	"context"
	"net"
	"net/http"
)

type RealIPCtxKey struct{}

func GetRealIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Real-IP")

		if ip == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr) //если ip пустой получаем ip отправляющего, порт не нужен(пропускаем)
			if err == nil {
				ip = host
			}
		}
		ctx := context.WithValue(r.Context(), RealIPCtxKey{}, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRealIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(RealIPCtxKey{}).(string)
	return ip
}
