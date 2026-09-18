package client

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/web/security"
)

func TestUploadReportRejectsOtherClientUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/report", func(c *gin.Context) {
		c.Set("client_uuid", "authenticated-node")
		c.Set("role", "client")
		UploadReport(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/report", strings.NewReader(`{"uuid":"other-node"}`)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-node report accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestCompressedReportSizeLimit(t *testing.T) {
	for _, size := range []int{128, int(security.MaxMessageBytes) + 1} {
		var compressed bytes.Buffer
		zw := gzip.NewWriter(&compressed)
		if _, err := zw.Write(bytes.Repeat([]byte("x"), size)); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/report", &compressed)
		req.Header.Set("Content-Encoding", "gzip")
		data, err := readMaybeCompressedBody(req)
		if (err != nil) != (size > int(security.MaxMessageBytes)) {
			t.Fatalf("size %d: %v", size, err)
		}
		if err == nil && len(data) != size {
			t.Fatal("normal report truncated")
		}
	}
}
