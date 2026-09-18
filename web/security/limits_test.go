package security

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReadLimited(t *testing.T) {
	for _, size := range []int{15, 16, 17} {
		data, err := ReadLimited(strings.NewReader(strings.Repeat("x", size)), 16)
		if (err != nil) != (size > 16) {
			t.Fatalf("size %d: %v", size, err)
		}
		if err == nil && len(data) != size {
			t.Fatal("payload was truncated")
		}
	}
}

func TestRequestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestBodyLimit())
	r.POST("/api/login", func(c *gin.Context) {
		if _, err := io.Copy(io.Discard, c.Request.Body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusNoContent)
	})
	for _, length := range []int64{-1, MaxMessageBytes + 1} {
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(strings.Repeat("x", int(MaxMessageBytes+1))))
		req.ContentLength = length
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("length %d: got %d", length, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"username":"admin"}`)))
	if w.Code != http.StatusNoContent {
		t.Fatalf("normal request: %d", w.Code)
	}
}
