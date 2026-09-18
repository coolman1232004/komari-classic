package terminal

import (
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/web/connection"
	"time"
)

func ForwardTerminal(id string) {
	TerminalSessionsMutex.Lock()
	s := TerminalSessions[id]
	TerminalSessionsMutex.Unlock()
	if s == nil || s.Agent == nil || s.Browser == nil {
		return
	}
	defer closeSession(id)
	started := time.Now()
	auditlog.Log(s.RequesterIp, s.UserUUID, "terminal opened for node:"+s.UUID, "terminal")
	defer func() {
		auditlog.Log(s.RequesterIp, s.UserUUID, "terminal closed for node:"+s.UUID+", duration:"+time.Since(started).String(), "terminal")
	}()
	done := make(chan struct{}, 2)
	valid := func() bool { return s.UserValid() != false && s.AgentValid() != false }
	copyMessages := func(src, dst *connection.SafeConn, browser bool) {
		defer func() { done <- struct{}{} }()
		for {
			kind, data, err := src.ReadMessage()
			if err != nil || !valid() {
				return
			}
			if !browser || kind != websocket.TextMessage || len(data) == 0 || data[0] != '{' {
				kind = websocket.BinaryMessage
			}
			if err := dst.WriteMessage(kind, data); err != nil {
				return
			}
		}
	}
	go copyMessages(s.Browser, s.Agent, true)
	go copyMessages(s.Agent, s.Browser, false)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if !valid() {
				return
			}
		}
	}
}
