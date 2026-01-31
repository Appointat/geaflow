package pool

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// UserConnection stores a single WebSocket connection and its metadata
type UserConnection struct {
	Conn       *websocket.Conn
	UserID     string
	LastActive time.Time
	writeMutex sync.Mutex // Protects concurrent writes to this connection
}

// SafeWriteJSON safely writes JSON to a single WebSocket connection (thread-safe)
func (uc *UserConnection) SafeWriteJSON(v interface{}) error {
	uc.writeMutex.Lock()
	defer uc.writeMutex.Unlock()
	return uc.Conn.WriteJSON(v)
}
