package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()

	_, subnet, err := net.ParseCIDR(cidr)
	require.NoError(t, err)
	return subnet
}

func TestTrustedSubnetHandler(t *testing.T) {
	tests := []struct {
		name       string
		subnet     *net.IPNet
		realIP     string
		wantStatus int
	}{
		{"ip inside subnet", mustParseCIDR(t, "192.168.1.0/24"), "192.168.1.42", http.StatusOK},
		{"ip on subnet edge", mustParseCIDR(t, "192.168.1.0/24"), "192.168.1.255", http.StatusOK},
		{"ip outside subnet", mustParseCIDR(t, "192.168.1.0/24"), "192.168.2.1", http.StatusForbidden},
		{"missing header", mustParseCIDR(t, "192.168.1.0/24"), "", http.StatusForbidden},
		{"malformed header", mustParseCIDR(t, "192.168.1.0/24"), "not-an-ip", http.StatusForbidden},
		{"ipv6 inside subnet", mustParseCIDR(t, "2001:db8::/32"), "2001:db8::1", http.StatusOK},
		{"ipv6 outside subnet", mustParseCIDR(t, "2001:db8::/32"), "2001:db9::1", http.StatusForbidden},
		{"nil subnet denies everyone", nil, "192.168.1.42", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			r := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
			if tt.realIP != "" {
				r.Header.Set("X-Real-IP", tt.realIP)
			}
			w := httptest.NewRecorder()

			TrustedSubnetHandler(tt.subnet)(next).ServeHTTP(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)
			assert.Equal(t, tt.wantStatus == http.StatusOK, nextCalled)
		})
	}
}
