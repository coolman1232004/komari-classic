package log

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLogsOmitCredentials(t *testing.T) {
	var output bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(GinLogger())
	r.GET("/api/clients/report", func(c *gin.Context) { c.Status(204) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/clients/report?token=secret-agent-token&otp=123456&Authorization=secret-key", nil))
	got := output.String()
	for _, secret := range []string{"secret-agent-token", "123456", "secret-key"} {
		if strings.Contains(got, secret) {
			t.Fatalf("credential leaked into log: %s", secret)
		}
	}
	if !strings.Contains(got, "/api/clients/report") {
		t.Fatal("request path missing")
	}
}
