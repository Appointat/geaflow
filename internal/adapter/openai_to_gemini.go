package adapter

import (
	"encoding/json"
	"log"

	"wsproxy/internal/multimodal"
)

// convertOpenAIMessagesToGeminiContents converts OpenAI messages format to Gemini contents format
func convertOpenAIMessagesToGeminiContents(messages []interface{}) []interface{} {
	contents := make([]interface{}, 0, len(messages))

	for _, msgInterface := range messages {
		msg, ok := msgInterface.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msg["role"].(string)
		content := msg["content"]

		// Create Gemini content object
		geminiContent := map[string]interface{}{}

		// Map OpenAI roles to Gemini roles
		switch role {
		case "system":
			geminiContent["role"] = "user"
		case "assistant":
			geminiContent["role"] = "model"
		case "user":
			geminiContent["role"] = "user"
		default:
			geminiContent["role"] = "user"
		}

		// Convert content to parts array
		var parts []interface{}

		switch v := content.(type) {
		case string:
			parts = []interface{}{
				map[string]interface{}{
					"text": v,
				},
			}
		case []interface{}:
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemType, _ := itemMap["type"].(string)

					if itemType == "text" {
						if text, ok := itemMap["text"].(string); ok {
							parts = append(parts, map[string]interface{}{
								"text": text,
							})
						}
					} else if itemType == "image_url" || itemType == "image" {
						if imageURLObj, ok := itemMap["image_url"].(map[string]interface{}); ok {
							if imageURL, ok := imageURLObj["url"].(string); ok {
								base64Data, mimeType, err := multimodal.FetchImageToBase64(imageURL)
								if err != nil {
									log.Printf("Failed to fetch image %s: %v", imageURL, err)
									continue
								}
								parts = append(parts, map[string]interface{}{
									"inline_data": map[string]interface{}{
										"mime_type": mimeType,
										"data":      base64Data,
									},
								})
							}
						}
					} else {
						parts = append(parts, itemMap)
					}
				}
			}
		default:
			parts = []interface{}{
				map[string]interface{}{
					"text": "",
				},
			}
		}

		geminiContent["parts"] = parts
		contents = append(contents, geminiContent)
	}

	return contents
}

// TransformRequest transforms OpenAI-style request to Gemini format
// For Gemini's OpenAI-compatible API, we DON'T convert messages to contents
// We only remove incompatible fields
func TransformRequest(bodyBytes []byte) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return bodyBytes
	}

	// Check if this is OpenAI format (has 'messages' field)
	if _, ok := payload["messages"].([]interface{}); ok {
		// For Gemini's OpenAI-compatible API, keep the messages format
		// Just remove fields that Gemini doesn't support

		delete(payload, "tool_choice") // Gemini doesn't use tool_choice
		delete(payload, "presence_penalty")
		delete(payload, "frequency_penalty")
		delete(payload, "logit_bias")
		delete(payload, "user")
		delete(payload, "n") // Gemini doesn't support multiple completions

		// Write modified payload back to bodyBytes
		if modifiedBody, err := json.Marshal(payload); err == nil {
			return modifiedBody
		} else {
			log.Printf("Warning: failed to re-marshal JSON after transformation: %v", err)
			return bodyBytes
		}
	}

	// If it's Gemini native format (contents), remove role fields
	if contents, ok := payload["contents"].([]interface{}); ok {
		for _, contentItem := range contents {
			if contentMap, ok := contentItem.(map[string]interface{}); ok {
				delete(contentMap, "role") // Key fix: remove role field
			}
		}

		// Write modified payload back to bodyBytes
		if modifiedBody, err := json.Marshal(payload); err == nil {
			return modifiedBody
		} else {
			log.Printf("Warning: failed to re-marshal JSON after transformation: %v", err)
			return bodyBytes
		}
	}

	return bodyBytes
}
