package debug

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"wsproxy/internal/config"
)

const (
	maxBodyLogLength = 5000 // Maximum chars to log for request/response bodies
	separator        = "==========================================="
)

// LogHTTPRequest logs the incoming HTTP request before any transformation
func LogHTTPRequest(r *http.Request, body []byte) {
	if !config.IsDebugMode() {
		return
	}

	fmt.Printf("\n[DEBUG] ============== HTTP REQUEST ==============\n")
	fmt.Printf("Method: %s\n", r.Method)
	fmt.Printf("URL: %s\n", r.URL.String())
	fmt.Printf("Headers:\n")
	for key, values := range r.Header {
		// Mask authorization headers for security
		if strings.ToLower(key) == "authorization" {
			fmt.Printf("  %s: %s***\n", key, values[0][:7])
		} else {
			for _, value := range values {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}

	if len(body) > 0 {
		truncated := TruncateJSON(body, maxBodyLogLength)
		fmt.Printf("Body (first %d chars):\n  %s\n", maxBodyLogLength, truncated)
	} else {
		fmt.Printf("Body: (empty)\n")
	}
	fmt.Printf("%s\n\n", separator)
}

// LogWSRequest logs the WebSocket request being sent to the client
func LogWSRequest(reqID string, msgType string, payload map[string]interface{}) {
	if !config.IsDebugMode() {
		return
	}

	fmt.Printf("\n[DEBUG] ============== WS REQUEST ==============\n")
	fmt.Printf("Request ID: %s\n", reqID)
	fmt.Printf("Type: %s\n", msgType)
	fmt.Printf("Payload:\n")

	payloadBytes, err := json.MarshalIndent(payload, "  ", "  ")
	if err != nil {
		fmt.Printf("  (error marshalling payload: %v)\n", err)
	} else {
		truncated := TruncateJSON(payloadBytes, maxBodyLogLength*2) // Allow more for WS messages
		fmt.Printf("  %s\n", truncated)
	}
	fmt.Printf("%s\n\n", separator)
}

// LogWSResponse logs the WebSocket response received from the client
func LogWSResponse(reqID string, msgType string, payload map[string]interface{}) {
	if !config.IsDebugMode() {
		return
	}

	fmt.Printf("\n[DEBUG] ============== WS RESPONSE ==============\n")
	fmt.Printf("Request ID: %s\n", reqID)
	fmt.Printf("Type: %s\n", msgType)
	fmt.Printf("Payload:\n")

	payloadBytes, err := json.MarshalIndent(payload, "  ", "  ")
	if err != nil {
		fmt.Printf("  (error marshalling payload: %v)\n", err)
	} else {
		truncated := TruncateJSON(payloadBytes, maxBodyLogLength*2)
		fmt.Printf("  %s\n", truncated)
	}
	fmt.Printf("%s\n\n", separator)
}

// LogHTTPResponse logs the final HTTP response being returned
func LogHTTPResponse(statusCode int, headers http.Header, body []byte) {
	if !config.IsDebugMode() {
		return
	}

	fmt.Printf("\n[DEBUG] ============== HTTP RESPONSE ==============\n")
	fmt.Printf("Status: %d %s\n", statusCode, http.StatusText(statusCode))
	fmt.Printf("Headers:\n")
	for key, values := range headers {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	if len(body) > 0 {
		truncated := TruncateJSON(body, maxBodyLogLength)
		fmt.Printf("Body (first %d chars):\n  %s\n", maxBodyLogLength, truncated)
	} else {
		fmt.Printf("Body: (empty)\n")
	}
	fmt.Printf("%s\n\n", separator)
}

// TruncateJSON truncates a JSON byte slice to maxLen characters
// Attempts to pretty-print first, but falls back to raw string if JSON is invalid
func TruncateJSON(data []byte, maxLen int) string {
	if len(data) == 0 {
		return "(empty)"
	}

	// Try to pretty-print JSON
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		prettyBytes, err := json.MarshalIndent(jsonData, "", "  ")
		if err == nil {
			data = prettyBytes
		}
	}

	str := string(data)
	if len(str) <= maxLen {
		return str
	}

	return str[:maxLen] + "... (truncated)"
}

// FormatHeaders formats HTTP headers for display
func FormatHeaders(headers http.Header) string {
	var sb strings.Builder
	for key, values := range headers {
		for _, value := range values {
			if strings.ToLower(key) == "authorization" {
				sb.WriteString(fmt.Sprintf("%s: %s***\n", key, value[:min(7, len(value))]))
			} else {
				sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
			}
		}
	}
	return sb.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
