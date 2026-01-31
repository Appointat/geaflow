package imagegen

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"wsproxy/internal/auth"
	"wsproxy/internal/pool"
	"wsproxy/internal/websocket"
)

// HandleImageGeneration returns an HTTP handler that proxies DALL-E style requests to Gemini Imagen
func HandleImageGeneration(connPool *pool.ConnectionPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Authenticate
		userID, err := auth.AuthenticateHTTPRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse OpenAI DALL-E format
		var dalleReq struct {
			Prompt         string `json:"prompt"`
			Model          string `json:"model"` // e.g., "dall-e-3"
			N              int    `json:"n"`     // number of images
			Size           string `json:"size"`  // e.g., "1024x1024"
			ResponseFormat string `json:"response_format"` // "url" or "b64_json"
		}

		if err := json.NewDecoder(r.Body).Decode(&dalleReq); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Convert to Gemini Imagen format
		imagenReq := map[string]interface{}{
			"instances": []map[string]interface{}{
				{
					"prompt": dalleReq.Prompt,
				},
			},
			"parameters": map[string]interface{}{
				"sampleCount": dalleReq.N,
				"aspectRatio": MapSizeToAspectRatio(dalleReq.Size),
			},
		}

		// Send to Gemini via WebSocket tunnel (similar to handleProxyRequest)
		reqID := uuid.NewString()
		respChan := make(chan *websocket.WSMessage, 10)
		websocket.PendingRequests.Store(reqID, respChan)
		defer websocket.PendingRequests.Delete(reqID)

		selectedConn, err := connPool.GetConnection(userID)
		if err != nil {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		bodyBytes, _ := json.Marshal(imagenReq)

		requestPayload := websocket.WSMessage{
			ID:   reqID,
			Type: websocket.TypeHTTPRequest,
			Payload: map[string]interface{}{
				"method":  "POST",
				"url":     "https://generativelanguage.googleapis.com/v1beta/models/imagen-3.0-generate-001:predict",
				"headers": map[string][]string{"Content-Type": {"application/json"}},
				"body":    string(bodyBytes),
			},
		}

		if err := selectedConn.SafeWriteJSON(requestPayload); err != nil {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}

		// Wait for response and convert to DALL-E format
		processImageGenerationResponse(w, r, respChan, dalleReq.ResponseFormat)
	}
}
