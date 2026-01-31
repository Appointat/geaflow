package multimodal

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"

	"wsproxy/internal/config"
)

// extractDataURL parses data URL format
// Format: data:image/jpeg;base64,<data>
func extractDataURL(dataURL string) (string, string, error) {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid data URL")
	}

	header := parts[0]
	data := parts[1]

	// Extract MIME type
	mimeType := "image/jpeg"
	if strings.HasPrefix(header, "data:") {
		mimePart := strings.TrimPrefix(header, "data:")
		mimePart = strings.Split(mimePart, ";")[0]
		if mimePart != "" {
			mimeType = mimePart
		}
	}

	return data, mimeType, nil
}

// FetchImageToBase64 fetches an image from URL and converts to base64
func FetchImageToBase64(imageURL string) (string, string, error) {
	// Validate URL
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	// Skip data URLs (already base64)
	if parsedURL.Scheme == "data" {
		// Extract mime type and data from data URL
		return extractDataURL(imageURL)
	}

	// Fetch via proxy
	resp, err := config.ProxyClient.Get(imageURL)
	if err != nil {
		return "", "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Size check
	if resp.ContentLength > config.MaxFileSize {
		return "", "", fmt.Errorf("file too large: %d bytes", resp.ContentLength)
	}

	// Read with size limit
	data, err := io.ReadAll(io.LimitReader(resp.Body, config.MaxFileSize+1))
	if err != nil {
		return "", "", fmt.Errorf("read failed: %w", err)
	}
	if len(data) > int(config.MaxFileSize) {
		return "", "", fmt.Errorf("file too large")
	}

	// Validate image format
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("invalid image: %w", err)
	}

	// Map format to MIME type
	mimeType := fmt.Sprintf("image/%s", format)
	if format == "jpeg" {
		mimeType = "image/jpeg"
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(data)

	return encoded, mimeType, nil
}
