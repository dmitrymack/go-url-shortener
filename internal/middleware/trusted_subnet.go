package middleware

import (
	"net"
	"net/http"
)

// TrustedSubnetHandler only lets through requests whose X-Real-IP is
// inside subnet (nil denies everyone) — client-spoofable, see README.
func TrustedSubnetHandler(subnet *net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := net.ParseIP(r.Header.Get("X-Real-IP"))
			if subnet == nil || ip == nil || !subnet.Contains(ip) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
