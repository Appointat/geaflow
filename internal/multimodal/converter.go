package multimodal

import (
	"encoding/json"
	"log"
	"strings"

	"wsproxy/internal/config"
)

// processMultimodalPart handles Gemini contents[].parts[] items
func processMultimodalPart(partMap map[string]interface{}) bool {
	// Check if inline_data already exists and is valid
	if inlineData, ok := partMap["inline_data"].(map[string]interface{}); ok {
		// Validate base64 data
		if data, ok := inlineData["data"].(string); ok {
			if len(data) > int(config.MaxFileSize) {
				log.Printf("Inline data too large: %d bytes", len(data))
				return false
			}
		}
		// Already in correct format
		return false
	}

	// Check for file_data (file URI references)
	if fileData, ok := partMap["file_data"].(map[string]interface{}); ok {
		if fileURI, ok := fileData["file_uri"].(string); ok {
			// If it's an HTTP(S) URL, fetch and convert
			if strings.HasPrefix(fileURI, "http://") || strings.HasPrefix(fileURI, "https://") {
				base64Data, mimeType, err := FetchImageToBase64(fileURI)
				if err != nil {
					log.Printf("Failed to fetch file %s: %v", fileURI, err)
					return false
				}

				delete(partMap, "file_data")
				partMap["inline_data"] = map[string]interface{}{
					"mime_type": mimeType,
					"data":      base64Data,
				}
				return true
			}
		}
	}

	return false
}

// processContentArray handles OpenAI messages[].content field
func processContentArray(msg map[string]interface{}, key string) bool {
	content := msg[key]

	// If content is string, no multimodal
	if _, ok := content.(string); ok {
		return false
	}

	// If content is array, process multimodal
	contentArray, ok := content.([]interface{})
	if !ok {
		return false
	}

	modified := false
	for _, item := range contentArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType, _ := itemMap["type"].(string)

		if itemType == "image_url" {
			// Convert OpenAI image_url to data URL format if not already
			if imageURLObj, ok := itemMap["image_url"].(map[string]interface{}); ok {
				if imageURL, ok := imageURLObj["url"].(string); ok {
					// Check if it's already a data URL
					if strings.HasPrefix(imageURL, "data:") {
						// Already in data URL format, keep as-is
						continue
					}

					// Fetch image and convert to data URL
					base64Data, mimeType, err := FetchImageToBase64(imageURL)
					if err != nil {
						log.Printf("Failed to fetch image %s: %v", imageURL, err)
						continue
					}

					// Update to data URL format
					imageURLObj["url"] = "data:" + mimeType + ";base64," + base64Data
					modified = true
				}
			}
		} else if itemType == "document" || itemType == "pdf" {
			// Handle PDF documents - convert to image_url format with data URL
			if docURL, ok := itemMap["document_url"].(string); ok {
				// Check if it's already a data URL
				if strings.HasPrefix(docURL, "data:") {
					// Already in data URL format, just convert structure
					delete(itemMap, "document_url")
					itemMap["type"] = "image_url"
					itemMap["image_url"] = map[string]interface{}{
						"url": docURL,
					}
					modified = true
					continue
				}

				// Fetch PDF and convert to data URL
				pdfData, err := FetchPDFToBase64(docURL)
				if err != nil {
					log.Printf("Failed to fetch PDF %s: %v", docURL, err)
					continue
				}

				// Convert to image_url format with data URL
				delete(itemMap, "document_url")
				itemMap["type"] = "image_url"
				itemMap["image_url"] = map[string]interface{}{
					"url": "data:application/pdf;base64," + pdfData,
				}
				modified = true
			}
		}
	}

	return modified
}

// ConvertMultimodalContent converts OpenAI format to Gemini format
// Handles both messages[] and contents[] arrays
func ConvertMultimodalContent(bodyBytes []byte) ([]byte, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return bodyBytes, nil // Not JSON, skip
	}

	modified := false

	// Process OpenAI-style messages
	if messages, ok := payload["messages"].([]interface{}); ok {
		for _, msgInterface := range messages {
			if msg, ok := msgInterface.(map[string]interface{}); ok {
				if processContentArray(msg, "content") {
					modified = true
				}
			}
		}
	}

	// Process Gemini-style contents
	if contents, ok := payload["contents"].([]interface{}); ok {
		for _, contentItem := range contents {
			if contentMap, ok := contentItem.(map[string]interface{}); ok {
				if parts, ok := contentMap["parts"].([]interface{}); ok {
					for _, partItem := range parts {
						if partMap, ok := partItem.(map[string]interface{}); ok {
							if processMultimodalPart(partMap) {
								modified = true
							}
						}
					}
				}
			}
		}
	}

	if !modified {
		return bodyBytes, nil
	}

	return json.Marshal(payload)
}
