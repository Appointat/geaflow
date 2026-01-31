package adapter

import (
	"encoding/json"
	"strings"
	"sync"

	"wsproxy/internal/cache"
)

// AdaptChunkToOpenAI fixes tool call indices and maps thinking process fields
// isThinking *bool is used to maintain state during streaming
func AdaptChunkToOpenAI(userID string, payload map[string]interface{}, isThinking *bool) map[string]interface{} {
	// 1. Extract data
	dataStr, ok := payload["data"].(string)
	if !ok || !strings.HasPrefix(dataStr, "data: ") {
		return payload // payload has no data string/not as expected, return as-is
	}
	jsonBodyStr := strings.TrimPrefix(dataStr, "data: ")

	// 2. Handle special cases like "data: [DONE]"
	if strings.TrimSpace(jsonBodyStr) == "[DONE]" {
		return payload
	}

	// 3. Parse JSON
	var chunkData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonBodyStr), &chunkData); err != nil {
		return payload
	}

	// 4. Deep dive into structure to find and fix tool_calls
	choices, choicesOk := chunkData["choices"].([]interface{})
	if !choicesOk || len(choices) == 0 {
		return payload
	}

	choice, choiceOk := choices[0].(map[string]interface{})
	if !choiceOk {
		return payload
	}

	delta, deltaOk := choice["delta"].(map[string]interface{})
	if !deltaOk {
		return payload
	}

	modified := false
	thoughtModified := false // Flag whether thought logic has been processed

	// --- A. Logic: Handle <thought> tags (Gemini 3 Flash/Pro Preview style) ---
	// Highest priority because content is in the content field
	if content, ok := delta["content"].(string); ok && content != "" {
		currentContent := content

		// 1. Detect start tag
		if strings.Contains(currentContent, "<thought>") {
			*isThinking = true
			currentContent = strings.Replace(currentContent, "<thought>", "", 1)
			thoughtModified = true
		}

		// 2. Detect end tag
		if strings.Contains(currentContent, "</thought>") {
			*isThinking = false
			parts := strings.Split(currentContent, "</thought>")

			// parts[0] is thought content (should go into reasoning_content)
			// parts[1] is main content (should remain in content)
			if len(parts) >= 2 {
				thoughtPart := parts[0]
				realContentPart := parts[1]

				if thoughtPart != "" {
					delta["reasoning_content"] = thoughtPart
				}
				delta["content"] = realContentPart // Update content to only contain main text
				currentContent = realContentPart   // Update current content variable for subsequent logic
				thoughtModified = true

				// Let program continue to check for tool_calls
			} else {
				// Case where there's only a tag without content
				currentContent = strings.Replace(currentContent, "</thought>", "", 1)
				thoughtModified = true
			}
		}

		// 3. Move field based on state
		if *isThinking {
			// If in thinking mode, move content to reasoning_content
			delta["reasoning_content"] = currentContent
			delta["content"] = "" // Clear original content to avoid displaying twice
			thoughtModified = true
		} else {
			// If tag was modified (stripped), write processed text back to content
			if thoughtModified {
				delta["content"] = currentContent
			}
		}
	}

	if thoughtModified {
		modified = true
	}

	// --- B. Handle thought in extra_content (for backward compatibility or specific configs) ---
	// Only attempt this if logic A wasn't triggered, to prevent conflicts
	if !thoughtModified {
		if extraContent, ok := delta["extra_content"].(map[string]interface{}); ok {
			if googleVal, ok := extraContent["google"].(map[string]interface{}); ok {
				if thought, ok := googleVal["thought"].(string); ok && thought != "" {
					delta["reasoning_content"] = thought
					modified = true
				}
			}
		}
	}

	// --- C. Logic: Fix tool_calls format (must always run) ---
	// a) Iterate tool_calls, add index field
	if toolCalls, toolCallsOk := delta["tool_calls"].([]interface{}); toolCallsOk {
		for i, tc := range toolCalls {
			if toolCall, ok := tc.(map[string]interface{}); ok {
				// 1. Key fix: add index
				if _, exists := toolCall["index"]; !exists {
					toolCall["index"] = float64(i)
					modified = true
				}

				// 2. Cache extra_content (user-level cache)
				if id, idOk := toolCall["id"].(string); idOk {
					if extraContent, ecOk := toolCall["extra_content"]; ecOk {
						userCache, _ := cache.UserToolCallCache.LoadOrStore(userID, &sync.Map{})
						userCache.(*sync.Map).Store(id, extraContent)
					}
				}
			}
		}
	}

	// --- D. Logic: Remove index field at same level as delta in choices[0] ---
	if _, exists := choice["index"]; exists {
		delete(choice, "index")
		modified = true
	}
	if !modified {
		return payload
	}

	// 5. Re-serialize
	fixedJSON, err := json.Marshal(chunkData)
	if err != nil {
		return payload
	}

	// 6. Update data field in payload
	payload["data"] = "data: " + string(fixedJSON)
	return payload
}
