package security

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestSensitiveResponsesCannotBeCached(t *testing.T) {
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/api/test", func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/test", nil))
	for k, v := range map[string]string{"Cache-Control": "no-store", "Referrer-Policy": "no-referrer", "X-Content-Type-Options": "nosniff"} {
		if w.Header().Get(k) != v {
			t.Fatalf("%s missing", k)
		}
	}
}
