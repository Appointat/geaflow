package websocket

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"wsproxy/internal/auth"
	"wsproxy/internal/pool"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// In production, should set strict CheckOrigin
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWebSocket returns an HTTP handler for WebSocket connections
func HandleWebSocket(connPool *pool.ConnectionPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Authentication
		authToken := r.URL.Query().Get("auth_token")
		userID, err := auth.ValidateJWT(authToken)
		if err != nil {
			log.Printf("WebSocket authentication failed: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Upgrade connection
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Failed to upgrade to WebSocket: %v", err)
			return
		}

		// Add to connection pool
		userConn := connPool.AddConnection(userID, conn)

		// Start read loop
		go readPump(userConn, connPool)
	}
}
