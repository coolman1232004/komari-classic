package log

import (
	"bytes"
	"context"
	"errors"
	gormlogger "gorm.io/gorm/logger"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestGormTraceRedactsSQLAndDriverError(t *testing.T) {
	var out bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)
	l := &GormLogger{LogLevel: gormlogger.Info}
	for _, err := range []error{nil, errors.New("secret-driver-value")} {
		l.Trace(context.Background(), time.Now(), func() (string, int64) { return "SELECT secret-query-value", 1 }, err)
	}
	if strings.Contains(out.String(), "secret-") || !strings.Contains(out.String(), "redacted") {
		t.Fatal(out.String())
	}
}
