# OpenAI 兼容模型路由器（本地代理）

一个**极简**本地 HTTP 代理服务：根据请求里的 `model` 字段，把请求转发到 `config.json` 中对应的上游（OpenAI 兼容接口），并将响应原样返回。

支持：

- `GET /v1/models`（OpenAI 风格）聚合所有模型
- **任意路径**透明转发（`/{path:path}`）
- **原样转发**请求与响应（支持 **SSE 流式**、**图片**、**PDF/二进制**、**multipart**）
- **自动剥离敏感请求头**（如 User-Agent, Origin, Sentry Trace）以规避上游风控

> 不做 transformer：本项目不会把 OpenAI 格式转换为 Anthropic/Gemini 等其他格式。

---

## 路由规则（按 model 选上游）

路由器需要知道“这次请求属于哪个模型”，从而选择 provider。取 `model` 的优先级：

1. Query 参数：`?model=xxx`
2. Header：`X-Model: xxx`
3. JSON Body：`{"model": "xxx"}`（仅当 `Content-Type: application/json` 时尝试解析）

---

## 项目结构

```text
router/
  server.py
  config.json
  pyproject.toml
  tests/
    test_router.py
```

---

## 安装依赖（uv）

安装运行依赖 + 测试依赖：

```bash
cd /Users/kuda/router
uv sync --extra dev
```

---

## 启动服务

```bash
cd /Users/kuda/router
# 开启 Debug 模式（可选）
ROUTER_DEBUG=1 uv run uvicorn server:app --host 127.0.0.1 --port 5840
```

健康检查：

```bash
curl http://127.0.0.1:5840/health
```

---

## 配置文件（config.json）

支持 **全局代理** 和 **Provider 级代理**。如果 Provider 设置了 `proxy_url`，优先使用；否则使用全局 `PROXY_URL`。

默认会剥离 `User-Agent`、`Origin` 等头。如需自定义剥离列表，可设置 `STRIP_HEADERS`。

示例：

```json
{
  "HOST": "127.0.0.1",
  "PORT": 5840,
  "APIKEY": "your-secret-key",
  "API_TIMEOUT_MS": 600000,
  "PROXY_URL": "http://127.0.0.1:7890",
  "Providers": [
    {
      "name": "p1",
      "api_base_url": "https://elysiver.h-e.top/v1/chat/completions",
      "api_key": "sk-xxx",
      "models": ["gpt-5.2"],
      "proxy_url": null
    },
    {
      "name": "local-model",
      "api_base_url": "http://127.0.0.1:1234/v1/chat/completions",
      "api_key": "",
      "models": ["llama-3-local"],
      "strip_headers": ["user-agent", "origin"]
    }
  ]
}
```

注意：

- `api_base_url` 必须是**完整上游 URL**（例如 `/v1/chat/completions`）。
- `api_key` 如果非空，路由器会覆盖上游 Authorization。
- `proxy_url` 留空则使用全局配置；设为 `""` 且全局也没配，则直连。

---

## 使用示例

### 1）列出所有模型

```bash
curl http://127.0.0.1:5840/v1/models \
  -H "Authorization: Bearer your-secret-key"
```

### 2）Chat Completions（JSON：model 在 body）

```bash
curl http://127.0.0.1:5840/v1/chat/completions \
  -H "Authorization: Bearer your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.2",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

### 3）流式输出（SSE）

```bash
curl -N http://127.0.0.1:5840/v1/chat/completions \
  -H "Authorization: Bearer your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.2",
    "messages": [{"role": "user", "content": "stream"}],
    "stream": true
  }'
```

### 4）PDF/二进制上传（推荐用 query 指定 model）

```bash
curl "http://127.0.0.1:5840/v1/files?model=gpt-5.2" \
  -H "Authorization: Bearer your-secret-key" \
  -H "Content-Type: application/pdf" \
  --data-binary @./demo.pdf
```

---

## 运行测试（pytest）

```bash
cd /Users/kuda/router
uv run --extra dev -m pytest -q
```

---

## Header 处理策略（当前实现）

为尽量规避上游风控/WAF，本路由器会对请求头做较强的剥离/改写。以下描述以 `server.py` 当前实现为准：

- **client -> router -> upstream（上游）**：
  - 以客户端的所有 headers 为起点（不是白名单策略）。
  - 永远移除 `HOP_BY_HOP` 列表中的头（包含：`Host`/`Connection`/`User-Agent`/`Sec-*`/`Accept-Encoding`/`Content-Length` 等）。
  - 额外移除 `strip_headers`：
    - 若 Provider 配置了 `strip_headers`（哪怕是空数组），则使用 Provider 的配置；
    - 否则使用全局 `STRIP_HEADERS`。
  - `Authorization`：若 Provider 配置了 `api_key`（且不为 `none/null/不需要apikey`），路由器会覆盖设置为 `Bearer <api_key>`；否则会移除该头（不会透传客户端 Authorization）。
  - 强制设置 `Content-Type: application/json`（用于兼容部分未显式设置 Content-Type 的客户端）。

- **upstream -> router -> client（响应）**：
  - 移除部分会导致客户端解码/连接语义混乱的头：`Content-Encoding`/`Transfer-Encoding`/`Content-Length`/`Connection`，以及 `HOP_BY_HOP` 中的头。
  - 其余上游响应头会透传给客户端。
  - 由于 `curl_cffi` 会自动解压内容，若把上游的 `Content-Encoding: gzip` 也透传给客户端，可能导致客户端二次解压报错，因此默认不透传 `Content-Encoding`。

> 备注：当前版本没有实现 `FORWARD_HEADERS` / `RESPONSE_HEADERS` 白名单配置；如需更严格/更可控的白名单策略，需要改代码。

## 常见问题排查

1. **上游 400 Bad Request**：
   - 检查 `api_base_url` 是否完整（例如是否少了 `/chat/completions`）。
   - 检查上游是否只支持 `/v1/completions`（`prompt` 字段）而不支持 chat。

2. **上游 403 Forbidden / 502 Bad Gateway**：
   - 可能是被 WAF 拦截。路由器已默认剥离 User-Agent，确保 `proxy_url` 配置正确。
   - 尝试开启 `ROUTER_DEBUG=1` 查看具体上游返回的 HTML body。

3. **Client 报错 429**：
   - 上游限流。路由器会将 429 及上游 body 原样透传给 client。
