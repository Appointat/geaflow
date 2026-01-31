package adapter

import (
	"encoding/json"
	"log"
)

// InjectThinkingConfig forcefully injects Gemini thinking mode configuration
func InjectThinkingConfig(bodyBytes []byte) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return bodyBytes
	}

	// 1. Check model name - only inject for models that support thinking (optional, we assume all models here)
	// model, _ := payload["model"].(string)
	// if !strings.Contains(model, "gemini") { return bodyBytes }

	// 2. Build google.thinking_config structure
	// Google's OpenAI-compatible interface requires non-standard parameters in extra_body,
	// or some libraries put them in the root-level google field.
	// Based on curl testing, placing in extra_body is effective.

	extraBody, ok := payload["extra_body"].(map[string]interface{})
	if !ok {
		extraBody = make(map[string]interface{})
	}

	googleConf, ok := extraBody["google"].(map[string]interface{})
	if !ok {
		googleConf = make(map[string]interface{})
	}

	thinkingConf, ok := googleConf["thinking_config"].(map[string]interface{})
	if !ok {
		thinkingConf = make(map[string]interface{})
	}

	// Force enable thinking
	thinkingConf["include_thoughts"] = true
	// thinkingConf["thinkingLevel"] = "high" // Optional, Gemini 3 supports it but Gemini 2.5 doesn't

	// Assemble back
	googleConf["thinking_config"] = thinkingConf
	extraBody["google"] = googleConf
	payload["extra_body"] = extraBody

	// 3. Re-serialize
	newBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error injecting thinking config: %v", err)
		return bodyBytes
	}

	log.Println("Successfully injected extra_body.google.thinking_config")
	return newBody
}
