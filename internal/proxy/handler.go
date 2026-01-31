package proxy

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"

	"wsproxy/internal/adapter"
	"wsproxy/internal/auth"
	"wsproxy/internal/cache"
	"wsproxy/internal/config"
	"wsproxy/internal/debug"
	"wsproxy/internal/multimodal"
	"wsproxy/internal/pool"
	"wsproxy/internal/websocket"
)

// HandleProxyRequest returns an HTTP handler for proxying requests through WebSocket
func HandleProxyRequest(connPool *pool.ConnectionPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Authenticate and get UserID
		log.Printf("--- 1. HTTP Request Received --- Method: %s, URL: %s", r.Method, r.URL.String())
		userID, err := auth.AuthenticateHTTPRequest(r)
		if err != nil {
			http.Error(w, "Proxy authentication failed", http.StatusUnauthorized)
			return
		}

		// 2. Generate unique request ID
		reqID := uuid.NewString()

		// 3. Create response channel and register
		// Use buffered channel to accommodate streaming response chunks
		respChan := make(chan *websocket.WSMessage, 10)
		websocket.PendingRequests.Store(reqID, respChan)
		defer websocket.PendingRequests.Delete(reqID) // Ensure cleanup after request ends

		// 4. Select a WebSocket connection
		selectedConn, err := connPool.GetConnection(userID)
		if err != nil {
			log.Printf("Error getting connection for user %s: %v", userID, err)
			http.Error(w, "Service Unavailable: No active client connected", http.StatusServiceUnavailable)
			return
		}

		// 5. Wrap HTTP request as WS message
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		// DEBUG: Log original HTTP request
		debug.LogHTTPRequest(r, bodyBytes)

		// Transform request body through pipeline:
		// 1. Inject thinking config
		bodyBytes = adapter.InjectThinkingConfig(bodyBytes)

		// 2. Convert multimodal content
		bodyBytes, err = multimodal.ConvertMultimodalContent(bodyBytes)
		if err != nil {
			log.Printf("Failed to convert multimodal content: %v", err)
			http.Error(w, "Failed to process multimodal content", http.StatusBadRequest)
			return
		}

		// 3. Inject tool call cache
		bodyBytes = cache.InjectExtraContentToToolCalls(userID, bodyBytes)

		// 4. Remove 'role' fields from contents array (Gemini compatibility)
		bodyBytes = adapter.TransformRequest(bodyBytes)

		// DEBUG: Save transformed request to file for inspection
		if config.IsDebugMode() {
			os.WriteFile("/tmp/transformed_request.json", bodyBytes, 0644)
			log.Printf("DEBUG: Transformed request saved to /tmp/transformed_request.json")
		}

		// Filter headers (remove HTTP/1.1 specific or proxy-specific headers)
		headers := make(map[string][]string)
		for k, v := range r.Header {
			if k != "Connection" && k != "Keep-Alive" && k != "Proxy-Authenticate" &&
				k != "Proxy-Authorization" && k != "Te" && k != "Trailers" &&
				k != "Transfer-Encoding" && k != "Upgrade" {
				headers[k] = v
			}
		}

		requestPayload := websocket.WSMessage{
			ID:   reqID,
			Type: websocket.TypeHTTPRequest,
			Payload: map[string]interface{}{
				"method":  r.Method,
				"url":     "https://generativelanguage.googleapis.com" + r.URL.String(),
				"headers": headers,
				"body":    string(bodyBytes),
			},
		}

		// DEBUG: Log WebSocket request being sent
		debug.LogWSRequest(reqID, requestPayload.Type, requestPayload.Payload)

		// 6. Send request to WebSocket client
		log.Printf("--- 2. Forwarding to WebSocket --- Request ID: %s", reqID)
		if err := selectedConn.SafeWriteJSON(requestPayload); err != nil {
			log.Printf("Failed to send request over WebSocket: %v", err)
			http.Error(w, "Bad Gateway: Failed to send request to client", http.StatusBadGateway)
			return
		}

		// 7. Asynchronously wait for and process response
		processWebSocketResponse(w, r, userID, reqID, respChan)
	}
}
