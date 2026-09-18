package terminal

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
	"net/http"
)

func EstablishConnection(c *gin.Context) {
	id := c.Query("id")
	TerminalSessionsMutex.Lock()
	s := TerminalSessions[id]
	if s == nil || s.Browser == nil {
		TerminalSessionsMutex.Unlock()
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if c.GetString("client_uuid") != s.UUID || s.claimed {
		TerminalSessionsMutex.Unlock()
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	s.claimed = true
	TerminalSessionsMutex.Unlock()
	valid := api.CredentialValidator(c)
	if !valid() {
		closeSession(id)
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	raw, err := api.UpgradeWebSocket(c)
	if err != nil {
		closeSession(id)
		return
	}
	conn := connection.NewSafeConn(raw)
	TerminalSessionsMutex.Lock()
	if TerminalSessions[id] != s {
		TerminalSessionsMutex.Unlock()
		conn.Close()
		return
	}
	s.Agent = conn
	s.AgentValid = valid
	TerminalSessionsMutex.Unlock()
	go ForwardTerminal(id)
}
