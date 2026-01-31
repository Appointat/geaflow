package proxy

import (
	"context"
	"log"
	"net/http"

	"wsproxy/internal/adapter"
	"wsproxy/internal/config"
	"wsproxy/internal/debug"
	"wsproxy/internal/websocket"
)

// processWebSocketResponse handles responses from WS channel and constructs HTTP response
func processWebSocketResponse(w http.ResponseWriter, r *http.Request, userID, reqID string, respChan chan *websocket.WSMessage) {
	defer log.Printf("--- 4. Final HTTP Response Sent --- Request URL: %s", r.URL.String())

	// Set timeout
	ctx, cancel := context.WithTimeout(r.Context(), config.ProxyRequestTimeout)
	defer cancel()

	// Get Flusher to support streaming responses
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("Warning: ResponseWriter does not support flushing, streaming will be buffered.")
	}

	headersSet := false
	var finalBody []byte
	var finalStatusCode int

	// --- NEW: Track thinking state ---
	isThinking := false
	// ------------------------------

	for {
		select {
		case msg, ok := <-respChan:
			log.Printf("--- 3. Response Received from WebSocket --- Request ID: %s, Type: %s", msg.ID, msg.Type)
			if !ok {
				// Channel closed, shouldn't happen unless there's a panic
				if !headersSet {
					http.Error(w, "Internal Server Error: Response channel closed unexpectedly", http.StatusInternalServerError)
				}
				return
			}

			// DEBUG: Log WebSocket response
			debug.LogWSResponse(reqID, msg.Type, msg.Payload)

			switch msg.Type {
			case websocket.TypeHTTPResponse:
				// Standard single response
				if headersSet {
					log.Println("Received http_response after headers were already set. Ignoring.")
					return
				}
				setResponseHeaders(w, msg.Payload)
				finalStatusCode = writeStatusCode(w, msg.Payload)
				finalBody = writeBody(w, msg.Payload)

				// DEBUG: Log final HTTP response
				debug.LogHTTPResponse(finalStatusCode, w.Header(), finalBody)
				return // Request finished

			case websocket.TypeStreamStart:
				// Stream start
				if headersSet {
					log.Println("Received stream_start after headers were already set. Ignoring.")
					continue
				}
				setResponseHeaders(w, msg.Payload)
				finalStatusCode = writeStatusCode(w, msg.Payload)
				headersSet = true
				if flusher != nil {
					flusher.Flush()
				}

			case websocket.TypeStreamChunk:
				// Stream data chunk
				if !headersSet {
					log.Println("Warning: Received stream_chunk before stream_start. Using default 200 OK.")
					w.WriteHeader(http.StatusOK)
					headersSet = true
				}

				// Adapt chunk format before writing to response
				msg.Payload = adapter.AdaptChunkToOpenAI(userID, msg.Payload, &isThinking)
				chunkBody := writeBody(w, msg.Payload)
				finalBody = append(finalBody, chunkBody...)

				if flusher != nil {
					flusher.Flush() // Immediately send chunk to client
				}

			case websocket.TypeStreamEnd:
				// Stream end
				if !headersSet {
					finalStatusCode = http.StatusOK
					w.WriteHeader(finalStatusCode)
				}

				// DEBUG: Log final HTTP response (streamed)
				debug.LogHTTPResponse(finalStatusCode, w.Header(), finalBody)
				return // Request finished

			case websocket.TypeError:
				// Error from client
				if !headersSet {
					errMsg := "Bad Gateway: Client reported an error"
					if payloadErr, ok := msg.Payload["error"].(string); ok {
						errMsg = payloadErr
					}
					statusCode := http.StatusBadGateway
					if code, ok := msg.Payload["status"].(float64); ok {
						statusCode = int(code)
					}
					http.Error(w, errMsg, statusCode)
				} else {
					log.Printf("Error received from client after stream started: %v", msg.Payload)
				}
				return // Request finished

			default:
				log.Printf("Received unexpected message type %s while waiting for response", msg.Type)
			}

		case <-ctx.Done():
			// Timeout
			if !headersSet {
				log.Printf("Gateway Timeout: No response from client for request %s", r.URL.Path)
				http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
			} else {
				log.Printf("Gateway Timeout: Stream incomplete for request %s", r.URL.Path)
			}
			return
		}
	}
}

// setResponseHeaders parses and sets HTTP response headers from payload
func setResponseHeaders(w http.ResponseWriter, payload map[string]interface{}) {
	headers, ok := payload["headers"].(map[string]interface{})
	if !ok {
		return
	}
	for key, value := range headers {
		// Assume value is []interface{} or string
		if values, ok := value.([]interface{}); ok {
			for _, v := range values {
				if strV, ok := v.(string); ok {
					w.Header().Add(key, strV)
				}
			}
		} else if strV, ok := value.(string); ok {
			w.Header().Set(key, strV)
		}
	}
}

// writeStatusCode parses and sets HTTP status code from payload
func writeStatusCode(w http.ResponseWriter, payload map[string]interface{}) int {
	status, ok := payload["status"].(float64) // JSON numbers are float64 by default
	if !ok {
		w.WriteHeader(http.StatusOK) // Default to 200 OK
		return http.StatusOK
	}
	intStatusCode := int(status)
	w.WriteHeader(intStatusCode)
	return intStatusCode
}

// writeBody parses and writes HTTP response body from payload
func writeBody(w http.ResponseWriter, payload map[string]interface{}) []byte {
	var bodyData []byte
	// For http_response, 'body' key usually contains data
	if body, ok := payload["body"].(string); ok {
		bodyData = []byte(body)
	}
	// For stream_chunk, 'data' key usually contains data
	if data, ok := payload["data"].(string); ok {
		bodyData = []byte(data)
	}

	if len(bodyData) > 0 {
		w.Write(bodyData)
		// SSE protocol requires newline separators for each event
		// For non-streaming responses, extra newlines are generally harmless
		w.Write([]byte("\n\n"))
	}
	return bodyData
}
