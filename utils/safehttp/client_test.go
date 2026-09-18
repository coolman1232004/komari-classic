package safehttp

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDenyInternalDestinations(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "0.0.0.0", "::1", "::ffff:127.0.0.1", "fe80::1", "fc00::1"} {
		if PublicIP(net.ParseIP(raw)) {
			t.Errorf("accepted %s", raw)
		}
	}
	if !PublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP rejected")
	}
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("SSRF reached server") }))
	defer s.Close()
	if _, err := Get(s.URL); err == nil {
		t.Fatal("loopback dial allowed")
	}
	for _, raw := range []string{"file:///etc/passwd", "http://user:pass@example.com"} {
		u, _ := url.Parse(raw)
		if validateURL(u) == nil {
			t.Fatal("unsafe URL allowed")
		}
	}
}
