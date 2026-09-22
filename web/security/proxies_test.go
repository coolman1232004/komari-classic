package security

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProxyAddressTrust(t *testing.T) {
	for _, tt := range []struct{ name, trusted, peer, forwarded, want string }{
		{"default rejects spoof", "", "192.0.2.10:1234", "198.51.100.4", "192.0.2.10"},
		{"untrusted rejects spoof", "192.0.2.20", "192.0.2.10:1234", "198.51.100.4", "192.0.2.10"},
		{"trusted proxy", "192.0.2.20", "192.0.2.20:1234", "198.51.100.4", "198.51.100.4"},
		{"trusted proxy second client", "192.0.2.20", "192.0.2.20:1234", "198.51.100.5", "198.51.100.5"},
		{"spoof preceding untrusted hop", "192.0.2.20", "192.0.2.20:1234", "203.0.113.99, 198.51.100.4", "198.51.100.4"},
		{"malformed header", "192.0.2.20", "192.0.2.20:1234", "bad-ip", "192.0.2.20"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			if err := ConfigureTrustedProxies(r, tt.trusted); err != nil {
				t.Fatal(err)
			}
			r.GET("/", func(c *gin.Context) { c.String(200, LoginClientAddress(c)) })
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.peer
			req.Header.Set("X-Forwarded-For", tt.forwarded)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Body.String() != tt.want {
				t.Fatalf("got %q, want %q", w.Body.String(), tt.want)
			}
		})
	}
}

func TestProxyConfigFailsClosed(t *testing.T) {
	for _, value := range []string{"0.0.0.0/0", "::/0", "*", "localhost", "192.0.2.1,"} {
		if err := ConfigureTrustedProxies(gin.New(), value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	r := gin.New()
	r.GET("/", func(c *gin.Context) { c.String(200, LoginClientAddress(c)) })
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.String() != req.RemoteAddr {
		t.Fatal("unconfigured router trusted a header")
	}
}
