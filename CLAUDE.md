# 代码分析报告：模块化 WebSocket 代理

## 项目概览

这是一个基于 Go 的 **WebSocket 代理服务器**，作为 HTTP 客户端和 WebSocket 客户端之间的隧道。它的主要功能是将来自 OpenAI 兼容客户端的 HTTP 请求转发到远程 WebSocket 客户端，然后由该客户端调用 Google Gemini API。服务器处理所有协议和格式转换。

### 使用场景

- **HTTP 客户端**：任何使用 OpenAI Completion API 兼容客户端库的应用程序（例如 OpenAI SDK）。
- **WebSocket 客户端**：远程客户端（例如浏览器或 Node.js），接收请求，调用 Gemini API，并流式返回响应。

### 核心价值

该代理允许任何 OpenAI 兼容的客户端无缝使用 Google Gemini API，提供：

- **协议转换**：将标准 HTTP 请求转换为自定义 WebSocket 协议。
- **格式兼容**：自动在 OpenAI 和 Gemini API 格式之间进行转换。
- **无 CORS 通信**：通过 WebSocket 绕过浏览器 CORS 限制。
- **流式支持**：完全支持 Server-Sent Events (SSE)。
- **多模态内容**：处理图像输入（Vision）、PDF 分析和图像生成（Imagen）。
- **可调试性**：内置调试模式，用于跟踪整个请求/响应流程。

## 核心架构

应用程序结构化为 `cmd` 入口点和多个 `internal` 包，促进关注点分离和可维护性。

### 目录结构

```text
aistudio-build-2-api/
├── cmd/
│   └── wsproxy/
│       └── main.go
├── internal/
│   ├── adapter/
│   ├── auth/
│   ├── cache/
│   ├── config/
│   ├── debug/
│   ├── imagegen/
│   ├── multimodal/
│   ├── pool/
│   ├── proxy/
│   ├── server/
│   └── websocket/
├── go.mod
├── go.sum
└── CLAUDE.md
```

---

### 1. 入口点 (`cmd/wsproxy/main.go`)

- **用途**：初始化应用程序并启动服务器。
- **关键操作**：
  - 解析命令行标志（例如 `-debug`）。
  - 初始化 `config` 包，读取环境变量（`DEBUG`）。
  - 调用 `server.Run()` 启动 HTTP 服务器。

---

### 2. 配置管理 (`internal/config`)

- **用途**：管理所有应用程序常量和配置。
- **关键组件**：
  - `config.go`：定义服务器地址、超时和文件大小常量。
  - 初始化共享的 `http.Client` 用于代理。
  - 管理 `DebugMode` 标志，可通过环境变量或命令行标志设置。

---

### 3. 调试功能 (`internal/debug`)

- **用途**：在启用调试模式时提供详细的、带颜色的日志记录，用于跟踪请求/响应流程。
- **关键函数**：
  - `LogHTTPRequest()`：记录原始传入的 HTTP 请求。
  - `LogWSRequest()`：记录正在转发的 WebSocket 消息。
  - `LogWSResponse()`：记录从客户端收到的 WebSocket 消息。
  - `LogHTTPResponse()`：记录最终发送回客户端的 HTTP 响应。
- **激活方式**：`DEBUG=true ./wsproxy` 或 `./wsproxy -debug`。

---

### 4. 连接池 (`internal/pool`)

- **用途**：管理所有来自客户端的活跃 WebSocket 连接。
- **关键组件**：
  - `connection.go`：定义 `UserConnection` 结构体，包装单个 WebSocket 连接。
  - `pool.go`：实现 `ConnectionPool`，使用 `sync.RWMutex` 安全地管理多个用户的连接。
  - **负载均衡**：使用轮询策略（`GetConnection`）在用户的可用连接之间分配请求。

---

### 5. WebSocket 处理 (`internal/websocket`)

- **用途**：管理 WebSocket 协议，包括升级和消息处理。
- **关键组件**：
  - `message.go`：定义 `WSMessage` 结构体和协议常量（例如 `TypeHTTPRequest`）。
  - `handler.go`：处理 HTTP 升级请求以建立 WebSocket 连接。
  - `pump.go`：为每个连接运行 `readPump` goroutine，持续读取消息、处理 ping 并路由响应。

---

### 6. HTTP 代理 (`internal/proxy`)

- **用途**：应用程序的核心；处理传入的 HTTP 请求并协调转换和转发过程。
- **关键组件**：
  - `handler.go`：接收请求的主 HTTP 处理程序。
    - **请求管道**：按顺序调用其他包来转换请求：
            1. `auth.AuthenticateHTTPRequest`
            2. `debug.LogHTTPRequest`（如果启用调试模式）
            3. `adapter.InjectThinkingConfig`
            4. `multimodal.ConvertMultimodalContent`
            5. `cache.InjectExtraContentToToolCalls`
            6. `adapter.TransformRequest`
    - 通过 `selectedConn.SafeWriteJSON()` 将转换后的请求转发到 WebSocket 客户端。
  - `response.go`：处理来自 WebSocket 客户端的响应。
    - `processWebSocketResponse`：监听通道以获取 `WSMessage` 响应。
    - 支持流式和标准响应。
    - 为每个流式块调用 `adapter.AdaptChunkToOpenAI`。
    - 在此处调用 `debug.LogWSResponse` 和 `debug.LogHTTPResponse`。

---

### 7. API 适配器 (`internal/adapter`)

- **用途**：处理在 OpenAI 和 Gemini API 格式之间转换的复杂任务。
- **关键组件**：
  - `openai_to_gemini.go`: `TransformRequest` 适配 OpenAI 兼容请求。它**保留**了 OpenAI 的 `messages` 格式，但移除了 Gemini 不支持的字段（如 `tool_choice`, `n` 等）。对于 Gemini 原生请求（`contents`），它会移除 `role` 字段。
  - `gemini_to_openai.go`：`AdaptChunkToOpenAI` 转换 Gemini 响应流。
    - 向 `tool_calls` 添加 `index` 字段。
    - 解析 `<thought>` 标签并将内容移动到 `reasoning_content` 字段。
    - 缓存来自工具调用的 `extra_content` 以供稍后注入。
  - `thinking.go`：将 `thinking_config` 注入请求以启用 Gemini 的推理能力。

---

### 8. 多模态与图像生成 (`internal/multimodal`, `internal/imagegen`)

- **用途**：扩展代理以处理非文本内容。
- **关键功能**：
  - `multimodal/image.go`：从 URL 获取和验证图像。
  - `multimodal/pdf.go`：获取和验证 PDF 文件。
  - `multimodal/converter.go`：将 `image_url` (图片) 和 `document_url` (PDF) 的 HTTP URL 转换为 Gemini OpenAI 兼容接口支持的 **Data URL** 格式 (`data:<mime_type>;base64,...`)。
  - `imagegen/handler.go`：模仿 DALL-E API 的专用端点（`/v1/images/generations`）。
  - `imagegen/transformer.go`：将 DALL-E 请求格式转换为 Gemini 的 Imagen API 格式，并将响应转换回来。

---

## 数据流

```
OpenAI 客户端 (HTTP)
        ↓
[proxy.HandleProxyRequest]
  - 认证请求
  - 记录 HTTP 请求（调试模式）
  - 转换：
    - 注入 thinking_config
    - 将文件 URL 转换为 Data URL
    - 注入工具缓存
    - 移除不兼容的 OpenAI 字段
  - 记录 WS 请求（调试模式）
        ↓
[WebSocket 隧道] → 远程 WS 客户端
                            ↓
                      [Gemini API]
                            ↓
[WebSocket 隧道] ← 远程 WS 客户端
        ↓
[proxy.processWebSocketResponse]
  - 记录 WS 响应（调试模式）
  - 适配块（Gemini 到 OpenAI）
  - 写入 HTTP 响应流
  - 记录最终 HTTP 响应（调试模式）
        ↓
OpenAI 客户端 (HTTP)
```

---

## 调试技巧与 API 兼容性要点

### 1. 高效的调试方法

- **基础调试日志 (`-debug`)**: 这是定位问题的起点，提供了基础的请求/响应流程信息。
- **突破日志长度限制**: 当默认日志长度不足以展示完整的 JSON 时，一个有效的方法是临时增大 `internal/debug/logger.go` 中的 `maxBodyLogLength` 常量。
- **文件转储 (File Dump)**: 最强大的调试方法是在请求发送前，将其完整地写入到临时文件。在 `internal/proxy/handler.go` 中加入 `os.WriteFile("/tmp/transformed_request.json", bodyBytes, 0644)`，可以提供一个绝对真实的请求快照，结合 `jq` 工具能清晰地分析其结构，这是定位复杂格式问题的关键。

### 2. Gemini OpenAI 兼容接口的“坑”

本次调试的核心在于理解 Gemini 的 **OpenAI 兼容接口** 的“方言”。它兼容，但**不完全相同**。

- **格式陷阱 (`messages` vs. `contents`)**: 最初我们错误地认为必须将 OpenAI 的 `messages` 格式转换为 Gemini 原生的 `contents` 格式。**真相是：兼容接口直接消费 `messages` 格式**，我们的转换是多余的。
- **文件/多模态陷阱 (`inline_data` vs. `data URL`)**: 兼容接口不使用 Gemini 原生的 `inline_data` 结构。它期望在标准的 `image_url` 字段中提供一个 **Data URL** (`data:<MIME_type>;base64,...`)。无论是图片还是 PDF，都必须以此格式提供。
- **参数裁剪的必要性**: 像 `tool_choice`, `n`, `presence_penalty` 等 OpenAI 参数在 Gemini 兼容接口中不受支持，必须**显式地从请求体中删除**，否则会触发 `400 Invalid Argument` 错误。
- **特殊功能 (`extra_body`)**: 像 `thinking_config` 这样的 Gemini 特有功能，是通过 OpenAI SDK 的 `extra_body` 字段传递的。代理需要正确地保留并转发这个结构。

---

## 变更总结

- **模块化**：原始的 1,318 行 `main.go` 文件已被拆分为 `internal/` 下的 **12 个不同的包**，提高了代码组织和可维护性。
- **清晰的入口点**：现在 `cmd/wsproxy/` 中存在一个最小的 `main.go`，仅负责初始化。
- **关注点分离**：每个包现在都有单一的职责（例如 `pool` 用于连接管理，`adapter` 用于格式转换）。
- **改进的可测试性**：使用新结构，包可以独立进行单元测试。
- **新的调试功能**：已添加全面的调试模式。可以通过 `DEBUG` 环境变量或 `-debug` 命令行标志激活，以提供整个请求/响应生命周期的详细跟踪。
