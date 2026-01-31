package multimodal

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"wsproxy/internal/config"
)

// FetchPDFToBase64 fetches a PDF from URL and converts to base64
func FetchPDFToBase64(pdfURL string) (string, error) {
	parsedURL, err := url.Parse(pdfURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "data" {
		// Extract from data URL
		data, _, err := extractDataURL(pdfURL)
		return data, err
	}

	resp, err := config.ProxyClient.Get(pdfURL)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if resp.ContentLength > config.MaxFileSize {
		return "", fmt.Errorf("file too large: %d bytes", resp.ContentLength)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, config.MaxFileSize+1))
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}

	if len(data) > int(config.MaxFileSize) {
		return "", fmt.Errorf("file too large")
	}

	// Validate PDF magic bytes
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return "", fmt.Errorf("not a valid PDF file")
	}

	return base64.StdEncoding.EncodeToString(data), nil
}
