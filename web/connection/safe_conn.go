package connection

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/web/security"
)

type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
	ID   int64
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{
		conn: conn,
		mu:   sync.Mutex{},
		ID:   time.Now().UnixNano(),
	}
}

func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return sc.conn.WriteMessage(messageType, data)
}

func (sc *SafeConn) WriteJSON(v interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return sc.conn.WriteJSON(v)
}

func (sc *SafeConn) Close() error {
	return sc.conn.Close()
}
func (sc *SafeConn) ReadMessage() (int, []byte, error) {
	messageType, reader, err := sc.conn.NextReader()
	if err != nil {
		return messageType, nil, err
	}
	// Gorilla's frame limit applies before permessage-deflate decompression.
	// Bound the expanded stream too, before allocating or decoding JSON.
	data, err := security.ReadLimited(reader, security.MaxMessageBytes)
	if err != nil {
		sc.conn.Close()
	}
	return messageType, data, err
}
func (sc *SafeConn) ReadJSON(v interface{}) error {
	_, data, err := sc.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
func (sc *SafeConn) SetReadDeadline(t time.Time) error {
	return sc.conn.SetReadDeadline(t)
}
func (sc *SafeConn) GetConn() *websocket.Conn {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn
}
