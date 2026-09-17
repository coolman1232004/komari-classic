package terminal

import (
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/web/connection"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTerminalEmptyFrameAndCredentialRevocation(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:terminal_security?mode=memory&cache=shared"
	accepted := make(chan *websocket.Conn, 2)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			accepted <- conn
		}
	}))
	defer server.Close()
	dial := func() (*websocket.Conn, *connection.SafeConn) {
		c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c, connection.NewSafeConn(<-accepted)
	}
	browser, serverBrowser := dial()
	agent, serverAgent := dial()
	var valid atomic.Bool
	valid.Store(true)
	TerminalSessionsMutex.Lock()
	TerminalSessions["forward-test"] = &TerminalSession{UUID: "node", Browser: serverBrowser, Agent: serverAgent, UserValid: valid.Load, AgentValid: func() bool { return true }}
	TerminalSessionsMutex.Unlock()
	defer closeSession("forward-test")
	done := make(chan struct{})
	go func() { ForwardTerminal("forward-test"); close(done) }()
	if err := browser.WriteMessage(websocket.TextMessage, []byte{}); err != nil {
		t.Fatal(err)
	}
	agent.SetReadDeadline(time.Now().Add(5 * time.Second))
	kind, data, err := agent.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage || len(data) != 0 {
		t.Fatalf("empty terminal frame: %d %q %v", kind, data, err)
	}
	valid.Store(false)
	if err := browser.WriteMessage(websocket.TextMessage, []byte("must not execute")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := agent.ReadMessage(); err == nil {
		t.Fatal("revoked user could send terminal input")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("terminal did not close")
	}
}
