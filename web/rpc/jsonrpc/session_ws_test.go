package jsonrpc

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	flags.DatabaseFile = "file:rpc_audit?mode=memory&cache=shared"
	flags.DatabaseType = flags.DatabaseTypeSQLite
	dbcore.GetDBInstance()
	os.Exit(m.Run())
}
func TestRevokedSessionLosesExistingWebSocketAuthority(t *testing.T) {
	u, err := accounts.CreateAccount("ws-audit", "password")
	if err != nil {
		t.Fatal(err)
	}
	token, err := accounts.CreateSession(u.UUID, 60, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(api.IdentityMiddleware())
	r.GET("/rpc", OnRpcRequest)
	s := httptest.NewServer(r)
	defer s.Close()
	h := http.Header{}
	h.Set("Cookie", "session_token="+token)
	h.Set("Origin", s.URL)
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/rpc", h)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	call := func() *rpc.JsonRpcResponse {
		t.Helper()
		if err := ws.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "admin:getSessions"}); err != nil {
			t.Fatal(err)
		}
		var result rpc.JsonRpcResponse
		if err := ws.ReadJSON(&result); err != nil {
			t.Fatal(err)
		}
		return &result
	}
	if call().Error != nil {
		t.Fatal("initial admin request denied")
	}
	if err := accounts.DeleteSession(token); err != nil {
		t.Fatal(err)
	}
	if call().Error == nil {
		t.Fatal("revoked session retained websocket privileges")
	}
}
