package websocket

import "sync"

// WSMessage is the basic structure for communication between frontend and backend
type WSMessage struct {
	ID      string                 `json:"id"`      // Unique ID for request/response
	Type    string                 `json:"type"`    // Message type: ping, pong, http_request, http_response, stream_start, stream_chunk, stream_end, error
	Payload map[string]interface{} `json:"payload"` // Message payload
}

// Message type constants
const (
	TypePing         = "ping"
	TypePong         = "pong"
	TypeHTTPRequest  = "http_request"
	TypeHTTPResponse = "http_response"
	TypeStreamStart  = "stream_start"
	TypeStreamChunk  = "stream_chunk"
	TypeStreamEnd    = "stream_end"
	TypeError        = "error"
)

// PendingRequests stores pending HTTP requests waiting for WS responses
// key: reqID (string), value: chan *WSMessage
var PendingRequests sync.Map
