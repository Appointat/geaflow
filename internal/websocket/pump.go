package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"wsproxy/internal/config"
	"wsproxy/internal/pool"
)

// readPump handles all incoming messages from a single WebSocket connection
func readPump(uc *pool.UserConnection, connPool *pool.ConnectionPool) {
	defer func() {
		connPool.RemoveConnection(uc.UserID, uc.Conn)
		uc.Conn.Close()
		log.Printf("readPump closed for user %s", uc.UserID)
	}()

	// Set read timeout (heartbeat mechanism)
	uc.Conn.SetReadDeadline(time.Now().Add(config.WSReadTimeout))

	for {
		_, message, err := uc.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error for user %s: %v", uc.UserID, err)
			} else {
				log.Printf("WebSocket closed for user %s: %v", uc.UserID, err)
			}
			// If read fails (including timeout), exit loop and clean up connection
			break
		}

		// On any message, reset read timeout
		uc.Conn.SetReadDeadline(time.Now().Add(config.WSReadTimeout))
		uc.LastActive = time.Now()

		// Parse message
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshalling WebSocket message: %v", err)
			continue
		}

		switch msg.Type {
		case TypePing:
			// Heartbeat response
			err := uc.SafeWriteJSON(map[string]string{"type": TypePong, "id": msg.ID})
			if err != nil {
				log.Printf("Error sending pong: %v", err)
				return // Send failed, assume connection is broken
			}
		case TypeHTTPResponse, TypeStreamStart, TypeStreamChunk, TypeStreamEnd, TypeError:
			// Route response to waiting HTTP Handler
			if ch, ok := PendingRequests.Load(msg.ID); ok {
				respChan := ch.(chan *WSMessage)
				// Try to send; if channel is full (unlikely, but for safety), log it
				select {
				case respChan <- &msg:
				default:
					log.Printf("Warning: Response channel full for request ID %s, dropping message type %s", msg.ID, msg.Type)
				}
			} else {
				log.Printf("Received response for unknown or timed-out request ID: %s", msg.ID)
			}
		default:
			log.Printf("Received unknown message type from client: %s", msg.Type)
		}
	}
}
