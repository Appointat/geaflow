package cache

import (
	"encoding/json"
	"log"
	"sync"
)

// UserToolCallCache is the user-level tool call cache
// Outer key is UserID, inner key is tool_call_id
var UserToolCallCache sync.Map

// InjectExtraContentToToolCalls checks for tool_call_id in the request body
// and injects the corresponding extra_content from the cache
func InjectExtraContentToToolCalls(userID string, bodyBytes []byte) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return bodyBytes // Not JSON, can't process
	}

	// 1. Find this user's dedicated cache
	userCacheInterface, found := UserToolCallCache.Load(userID)
	if !found {
		return bodyBytes // User has no cache, return as-is
	}
	userCache := userCacheInterface.(*sync.Map)

	modified := false

	// Check for OpenAI-style `messages`
	if messages, ok := payload["messages"].([]interface{}); ok {
		for _, msgInterface := range messages {
			if msg, ok := msgInterface.(map[string]interface{}); ok {
				if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
					for _, tcInterface := range toolCalls {
						if toolCall, tcOk := tcInterface.(map[string]interface{}); tcOk {
							if id, idOk := toolCall["id"].(string); idOk {
								// Load from user cache
								if extraContent, foundInCache := userCache.Load(id); foundInCache {
									toolCall["extra_content"] = extraContent
									log.Printf("[UserID: %s] Injected extra_content for tool_call_id: %s", userID, id)
									modified = true
									// Successfully injected, optionally delete from cache to avoid reuse
									// userCache.Delete(id)
								}
							}
						}
					}
				}
			}
		}
	}

	// Check for Gemini-style `contents`
	if contents, ok := payload["contents"].([]interface{}); ok {
		for _, contentItem := range contents {
			if contentMap, ok := contentItem.(map[string]interface{}); ok {
				if parts, ok := contentMap["parts"].([]interface{}); ok {
					for _, partItem := range parts {
						if partMap, ok := partItem.(map[string]interface{}); ok {
							if toolCalls, ok := partMap["tool_calls"].([]interface{}); ok {
								for _, tcInterface := range toolCalls {
									if toolCall, tcOk := tcInterface.(map[string]interface{}); tcOk {
										if id, idOk := toolCall["id"].(string); idOk {
											// Load from user cache
											if extraContent, foundInCache := userCache.Load(id); foundInCache {
												toolCall["extra_content"] = extraContent
												log.Printf("[UserID: %s] Injected extra_content for tool_call_id: %s", userID, id)
												modified = true
												// Successfully injected, optionally delete from cache to avoid reuse
												// userCache.Delete(id)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !modified {
		return bodyBytes
	}

	fixedBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[UserID: %s] Error re-marshalling body after injecting extra_content, returning original. Error: %v", userID, err)
		return bodyBytes
	}

	log.Printf("[UserID: %s] Successfully injected extra_content into request body.", userID)
	return fixedBody
}
