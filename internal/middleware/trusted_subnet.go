package middleware

import (
	"net"
	"net/http"
)

// TrustedSubnetHandler returns middleware that lets a request through only
// if its X-Real-IP header holds an address inside subnet, and responds 403
// otherwise (including for a missing or malformed header). A nil subnet
// denies every request.
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
