package terminal

import (
	"github.com/komari-monitor/komari/web/connection"
	"sync"
)

type TerminalSession struct {
	UUID, UserUUID, RequesterIp string
	Browser, Agent              *connection.SafeConn
	UserValid, AgentValid       func() bool
	claimed                     bool
}

var TerminalSessionsMutex = &sync.Mutex{}
var TerminalSessions = make(map[string]*TerminalSession)

func closeSession(id string) {
	TerminalSessionsMutex.Lock()
	session := TerminalSessions[id]
	delete(TerminalSessions, id)
	var browser, agent *connection.SafeConn
	if session != nil {
		browser, agent = session.Browser, session.Agent
	}
	TerminalSessionsMutex.Unlock()
	if browser != nil {
		browser.Close()
	}
	if agent != nil {
		agent.Close()
	}
}
