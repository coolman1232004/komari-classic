package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	v1 "github.com/komari-monitor/komari/protocol/v1"
	agent "github.com/komari-monitor/komari/web/agent"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLiveDataRefreshesVisibilityAndRevokedSessions(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:live_security?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if err := config.Set(config.PrivateSiteKey, false); err != nil {
		t.Fatal(err)
	}
	defer config.Set(config.PrivateSiteKey, false)
	node := models.Client{UUID: "live-security", Token: "live-token", Name: "node"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	agent.SetLatestReport(node.UUID, &v1.Report{UUID: node.UUID})
	r := gin.New()
	r.Use(IdentityMiddleware())
	r.GET("/api/clients", GetClients)
	server := httptest.NewServer(r)
	defer server.Close()
	dial := func(session string) *websocket.Conn {
		h := http.Header{"Origin": []string{server.URL}}
		if session != "" {
			h.Set("Cookie", "session_token="+session)
		}
		c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/clients", h)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	read := func(c *websocket.Conn) int {
		if err := c.WriteMessage(websocket.TextMessage, []byte("get")); err != nil {
			t.Fatal(err)
		}
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		var res struct {
			Data struct {
				Data map[string]v1.Report `json:"data"`
			} `json:"data"`
		}
		if err := c.ReadJSON(&res); err != nil {
			t.Fatal(err)
		}
		return len(res.Data.Data)
	}
	guest := dial("")
	if read(guest) != 1 {
		t.Fatal("public telemetry missing")
	}
	if err := db.Model(&node).Update("hidden", true).Error; err != nil {
		t.Fatal(err)
	}
	if read(guest) != 0 {
		t.Fatal("newly hidden node leaked to old guest connection")
	}
	user, err := accounts.CreateAccount("live-admin", "password")
	if err != nil {
		t.Fatal(err)
	}
	session, err := accounts.CreateSession(user.UUID, 60, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	admin := dial(session)
	if read(admin) != 1 {
		t.Fatal("admin cannot view hidden node")
	}
	accounts.DeleteSession(session)
	if read(admin) != 0 {
		t.Fatal("revoked admin still sees hidden node")
	}
	if err := config.Set(config.PrivateSiteKey, true); err != nil {
		t.Fatal(err)
	}
	guest.WriteMessage(websocket.TextMessage, []byte("get"))
	if _, _, err := guest.ReadMessage(); err == nil {
		t.Fatal("existing guest still accesses private site")
	}
}
