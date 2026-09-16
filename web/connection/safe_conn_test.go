package connection

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/web/security"
)

func TestCompressedMessageExpandedLimit(t *testing.T) {
	for _, size := range []int{128, int(security.MaxMessageBytes) + 1} {
		result := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{EnableCompression: true}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				result <- err
				return
			}
			defer conn.Close()
			conn.SetReadLimit(security.MaxMessageBytes)
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, _, err = NewSafeConn(conn).ReadMessage()
			result <- err
		}))
		dialer := websocket.Dialer{EnableCompression: true}
		conn, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		err = conn.WriteMessage(websocket.TextMessage, bytes.Repeat([]byte("x"), size))
		if err != nil {
			conn.Close()
			server.Close()
			t.Fatal(err)
		}
		select {
		case err = <-result:
			if (err != nil) != (size > int(security.MaxMessageBytes)) {
				t.Errorf("expanded size %d: %v", size, err)
			}
		case <-time.After(15 * time.Second):
			t.Error("reader did not terminate")
		}
		conn.Close()
		server.Close()
	}
}
