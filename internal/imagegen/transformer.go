package imagegen

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"wsproxy/internal/config"
	"wsproxy/internal/websocket"
)

// MapSizeToAspectRatio converts DALL-E size to Imagen aspect ratio
func MapSizeToAspectRatio(size string) string {
	switch size {
	case "1024x1024", "512x512", "256x256":
		return "1:1"
	case "1792x1024":
		return "16:9"
	case "1024x1792":
		return "9:16"
	default:
		return "1:1"
	}
}

// processImageGenerationResponse converts Imagen response to DALL-E format
func processImageGenerationResponse(w http.ResponseWriter, r *http.Request, respChan chan *websocket.WSMessage, responseFormat string) {
	ctx, cancel := context.WithTimeout(r.Context(), config.ProxyRequestTimeout)
	defer cancel()

	select {
	case msg := <-respChan:
		if msg.Type == websocket.TypeError {
			http.Error(w, "Image generation failed", http.StatusInternalServerError)
			return
		}

		// Parse Imagen response
		var imagenResp struct {
			Predictions []struct {
				BytesBase64Encoded string `json:"bytesBase64Encoded"`
				MimeType           string `json:"mimeType"`
			} `json:"predictions"`
		}

		if body, ok := msg.Payload["body"].(string); ok {
			json.Unmarshal([]byte(body), &imagenResp)
		}

		// Convert to DALL-E format
		dalleResp := map[string]interface{}{
			"created": time.Now().Unix(),
			"data":    []map[string]interface{}{},
		}

		for _, pred := range imagenResp.Predictions {
			item := map[string]interface{}{}
			if responseFormat == "b64_json" {
				item["b64_json"] = pred.BytesBase64Encoded
			} else {
				// For URL format, would need to upload to storage
				// For now, return as data URL
				item["url"] = fmt.Sprintf("data:%s;base64,%s", pred.MimeType, pred.BytesBase64Encoded)
			}
			dalleResp["data"] = append(dalleResp["data"].([]map[string]interface{}), item)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dalleResp)

	case <-ctx.Done():
		http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
	}
}
