# LLM API Reference & Compatibility Guide (2026)

**Last Updated:** 2026-01-09
**Context:** Reference for `openai-to-gemini` proxy development.

This document collects the latest API specifications for OpenAI (v1) and Google Gemini (v1beta/v1alpha) as of early 2026, focusing on changes relevant to proxy translation, reasoning models, and structured outputs.

---

## 1. OpenAI API (Latest)

### Core Endpoint: Chat Completions
**URL:** `POST https://api.openai.com/v1/chat/completions`

#### Key Request Parameters (2026 Updates)

```json
{
  "model": "gpt-4o", // or "o1", "o3-mini"
  "messages": [
    {
      "role": "system",
      "content": "You are a helper."
    },
    {
      "role": "user",
      "content": [
        { "type": "text", "text": "Analyze this image." },
        { "type": "image_url", "image_url": { "url": "..." } }
      ]
    }
  ],
  "store": true, // New: Store output for distillation/evals
  "reasoning_effort": "medium", // Options: "low", "medium", "high" (for o1/o3 models)
  "response_format": {
    "type": "json_schema", // Structured Outputs
    "json_schema": {
      "name": "math_response",
      "strict": true,
      "schema": { ... }
    }
  },
  "stream": true,
  "stream_options": {
    "include_usage": true
  }
}
```

#### Key Response Fields (Streamed)

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion.chunk",
  "choices": [
    {
      "index": 0,
      "delta": {
        "role": "assistant",
        "content": "The answer is...",
        "reasoning_content": "First I will calculate..." // O-series reasoning output
      },
      "finish_reason": null
    }
  ],
  "usage": { ... } // Sent in final chunk if stream_options.include_usage=true
}
```

> **Note:** The new **Responses API** (`/v1/responses`) is replacing Chat Completions for agentic workflows, featuring stateful context and `output_text` instead of `choices`.

---

## 2. Google Gemini API (Latest)

### Core Endpoint: Generate Content
**URL:** `POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent`
**URL (Stream):** `POST .../models/{model}:streamGenerateContent`

**Latest Models:**
- `gemini-2.5-pro`, `gemini-2.5-flash`
- `gemini-3-pro-preview`, `gemini-3-flash-preview` (New "Thinking" capabilities)

#### Request Structure (REST)

```json
{
  "contents": [
    {
      "role": "user", // "user" or "model" (no "system" inside contents)
      "parts": [
        { "text": "Explain quantum physics." }
      ]
    }
  ],
  "systemInstruction": {
    "parts": [ { "text": "You are a physics expert." } ]
  },
  "tools": [
    {
      "functionDeclarations": [ ... ]
    }
  ],
  "generationConfig": {
    "temperature": 1.0,
    "thinkingConfig": {
      "includeThoughts": true, // Enable reasoning output
      "thinkingLevel": "high"  // "low", "medium", "high", "minimal" (Flash only)
    }
  }
}
```

#### Response Structure (Streamed)

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          { "text": "Step 1: Analyze..." }, // Standard text
          { 
            "text": "I should verify this...", 
            "thought": true // Boolean flag indicating reasoning content
          } 
        ]
      },
      "finishReason": "STOP",
      "thoughtSignature": "sig_..." // Required for multi-turn reasoning context
    }
  ]
}
```

---

## 3. Proxy Translation Guide (OpenAI ↔ Gemini)

This section maps the fields for the `openai-to-gemini` proxy based on the analysis of `main.go`.

### A. Role Mapping
| OpenAI Role | Gemini Mapping | Notes |
| :--- | :--- | :--- |
| `system` | `systemInstruction` field | Extracted from messages array and moved to top-level config. |
| `user` | `role: "user"` | Direct mapping. |
| `assistant` | `role: "model"` | Direct mapping. |
| `tool` | `role: "function"` | Gemini uses `functionResponse` part type within `user` role usually, or distinct handling. |

### B. Reasoning & Thinking
The proxy needs to convert between OpenAI's `reasoning_content` and Gemini's `thinking_config`.

| Feature | OpenAI (Request) | Gemini (Request) |
| :--- | :--- | :--- |
| **Enable** | Auto (for o-series) | `generationConfig.thinkingConfig.includeThoughts = true` |
| **Effort** | `reasoning_effort` | `generationConfig.thinkingConfig.thinkingLevel` |

| Feature | OpenAI (Response) | Gemini (Response) |
| :--- | :--- | :--- |
| **Content** | `delta.reasoning_content` | `part.text` where `part.thought == true` |
| **Tags** | N/A (Field based) | **Legacy/Raw:** `<thought>...</thought>` (Proxy currently parses this) <br> **Modern:** `part.thought` boolean. |

**Proxy Logic Update Needed:**
If the proxy sees `part.thought == true` in the Gemini response, it should map that text to OpenAI's `choices[0].delta.reasoning_content` instead of `content`.

### C. Tool Calling (Function Calling)

**OpenAI Request:**
```json
"tools": [{ "type": "function", "function": { ... } }]
```

**Gemini Request:**
```json
"tools": [{ "functionDeclarations": [ ... ] }]
```

**OpenAI Response (Streamed):**
```json
"tool_calls": [{ "index": 0, "id": "call_123", "function": { "name": "...", "arguments": "..." } }]
```

**Gemini Response:**
```json
"parts": [{ "functionCall": { "name": "...", "args": { ... } } }]
```

**Critical Proxy Handling:**
1.  **ID Injection:** Gemini does not return a `tool_call_id` in the response. The proxy must generate one (e.g., `call_<uuid>`) and cache it (`userToolCallCache`) to map the subsequent `tool_output` back to the correct call.
2.  **Index:** OpenAI requires an `index` for tool calls in streams; Gemini does not. The proxy must artificially inject `index: 0`.

### D. Thought Signatures (Gemini 3 Special)
Gemini 3 models return a `thoughtSignature` string.
*   **Requirement:** If a user sends a follow-up message after a reasoning response, this signature **must** be passed back to Gemini to maintain the "train of thought".
*   **Proxy Action:** The proxy may need to store this in a hidden field or append it to the conversation history if the OpenAI client doesn't support opaque tokens.

---

## 4. Current Model Compatibility List

| Abstract Name | Recommended OpenAI Model | Recommended Gemini Model |
| :--- | :--- | :--- |
| **Fast / Cheap** | `gpt-4o-mini` | `gemini-2.5-flash` |
| **High Intellect** | `gpt-4o` | `gemini-2.5-pro` |
| **Reasoning** | `o1`, `o3-mini` | `gemini-3-flash-preview` (set `thinkingLevel: medium`) |
| **Deep Research** | `o3` | `gemini-3-pro-preview` (set `thinkingLevel: high`) |
