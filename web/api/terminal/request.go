package terminal

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/utils"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
	"net/http"
	"time"
)

func RequestTerminal(c *gin.Context) {
	uuid := c.Param("uuid")
	if _, err := clients.GetClientByUUID(uuid); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	valid := api.CredentialValidator(c)
	if !valid() {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	raw, err := api.UpgradeWebSocket(c)
	if err != nil {
		return
	}
	conn := connection.NewSafeConn(raw)
	id := utils.GenerateRandomString(32)
	session := &TerminalSession{UUID: uuid, UserUUID: c.GetString("uuid"), Browser: conn, RequesterIp: c.ClientIP(), UserValid: valid}
	TerminalSessionsMutex.Lock()
	TerminalSessions[id] = session
	TerminalSessionsMutex.Unlock()
	agent := agent_runtime.GetConnectedClients()[uuid]
	if agent == nil {
		conn.WriteMessage(1, []byte("Client offline"))
		closeSession(id)
		return
	}
	conn.WriteMessage(1, []byte("Waiting for agent..."))
	if err := agent.WriteJSON(gin.H{"message": "terminal", "request_id": id}); err != nil {
		closeSession(id)
		return
	}
	time.AfterFunc(30*time.Second, func() {
		TerminalSessionsMutex.Lock()
		s := TerminalSessions[id]
		pending := s != nil && s.Agent == nil
		TerminalSessionsMutex.Unlock()
		if pending {
			closeSession(id)
		}
	})
}
