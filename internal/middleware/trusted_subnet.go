package middleware

import (
	"net"
	"net/http"
)

func TrustedSubnetMiddleware(cidr string) func(http.Handler) http.Handler {
	if cidr == "" { // если пусто выходим
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil { // если конфиг битый
		panic("invalid trusted_subnet CIDR: " + cidr)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ipStr := GetRealIPFromContext(r.Context())
			if ipStr == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			ip := net.ParseIP(ipStr)
			if ip == nil || !subnet.Contains(ip) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
