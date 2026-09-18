package terminal

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/web/connection"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrongAgentCannotClaimTerminal(t *testing.T) {
	TerminalSessionsMutex.Lock()
	TerminalSessions["secret"] = &TerminalSession{UUID: "target", Browser: &connection.SafeConn{}}
	TerminalSessionsMutex.Unlock()
	defer func() {
		TerminalSessionsMutex.Lock()
		delete(TerminalSessions, "secret")
		TerminalSessionsMutex.Unlock()
	}()
	r := gin.New()
	r.GET("/terminal", func(c *gin.Context) { c.Set("client_uuid", "other"); EstablishConnection(c) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/terminal?id=secret", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("wrong agent got %d", w.Code)
	}
	TerminalSessionsMutex.Lock()
	claimed := TerminalSessions["secret"].claimed
	TerminalSessionsMutex.Unlock()
	if claimed {
		t.Fatal("wrong agent consumed session")
	}
}
