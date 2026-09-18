package security

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const MaxMessageBytes int64 = 8 << 20

// ReadLimited bounds the expanded size as well as the wire size of reports.
func ReadLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("message exceeds %d bytes", limit)
	}
	return data, nil
}

// RequestBodyLimit runs before identity parsing. File uploads have a larger
// streaming allowance; their handlers still require administrator authentication.
func RequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := MaxMessageBytes
		switch c.Request.URL.Path {
		case "/api/admin/upload/backup", "/api/admin/theme/upload":
			limit = 512 << 20
		}
		if c.Request.ContentLength > limit {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
