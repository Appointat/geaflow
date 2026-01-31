# 多模态功能说明

## 功能概述

此 WebSocket 代理服务器已扩展支持多模态内容处理，包括图片、PDF 文档和图片生成功能。所有功能都兼容 OpenAI API 格式，并自动转换为 Gemini API 格式。

## 核心功能

### 1. 图片输入 (Vision API)

支持在对话中发送图片进行分析。

#### 支持的图片格式
- **URL 格式**: HTTP/HTTPS 图片链接
- **Data URL 格式**: `data:image/png;base64,...`
- **图片类型**: JPEG, PNG, GIF
- **大小限制**: 50MB

#### 工作原理
1. 客户端发送 OpenAI 格式的请求（带 `image_url`）
2. 代理通过配置的 VPN 代理 (127.0.0.1:7890) 获取图片
3. 图片被转换为 base64 编码
4. 转换为 Gemini `inline_data` 格式
5. 发送到 Gemini API
6. 响应返回给客户端

#### 请求示例

```bash
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "这张图片里有什么？"},
        {
          "type": "image_url",
          "image_url": {
            "url": "https://example.com/image.jpg"
          }
        }
      ]
    }]
  }'
```

#### Data URL 示例

```bash
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "描述这张图片"},
        {
          "type": "image_url",
          "image_url": {
            "url": "data:image/png;base64,iVBORw0KGgo..."
          }
        }
      ]
    }]
  }'
```

### 2. PDF 文档输入

支持上传 PDF 文档进行分析和摘要。

#### 支持的 PDF 格式
- **URL 格式**: HTTP/HTTPS PDF 链接
- **Data URL 格式**: `data:application/pdf;base64,...`
- **大小限制**: 50MB
- **验证**: 必须包含 PDF 魔术字节 (`%PDF`)

#### 请求示例

```bash
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "总结这份文档"},
        {
          "type": "pdf",
          "document_url": "https://example.com/document.pdf"
        }
      ]
    }]
  }'
```

#### 混合内容示例

```bash
# 同时分析文本、图片和 PDF
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "对比这些内容的异同："},
        {"type": "image_url", "image_url": {"url": "https://example.com/chart.png"}},
        {"type": "pdf", "document_url": "https://example.com/report.pdf"}
      ]
    }]
  }'
```

### 3. 图片生成 (DALL-E 兼容 API)

支持使用 DALL-E 风格的 API 生成图片，实际使用 Gemini Imagen API。

#### 端点
```
POST /v1/images/generations
```

#### 支持的参数
- `prompt` (必需): 图片描述文本
- `n` (可选): 生成图片数量，默认 1
- `size` (可选): 图片尺寸
  - `1024x1024` (1:1) - 默认
  - `1792x1024` (16:9) - 宽屏
  - `1024x1792` (9:16) - 竖屏
  - `512x512` (1:1)
  - `256x256` (1:1)
- `response_format` (可选): 响应格式
  - `url` - 返回 data URL (默认)
  - `b64_json` - 返回纯 base64 数据

#### 请求示例

```bash
# 生成单张图片 - URL 格式
curl -X POST http://localhost:5345/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A cute cat playing piano",
    "n": 1,
    "size": "1024x1024",
    "response_format": "url"
  }'
```

#### 响应示例

```json
{
  "created": 1704835200,
  "data": [
    {
      "url": "data:image/png;base64,iVBORw0KGgo..."
    }
  ]
}
```

或使用 `b64_json` 格式：

```json
{
  "created": 1704835200,
  "data": [
    {
      "b64_json": "iVBORw0KGgo..."
    }
  ]
}
```

## 技术架构

### 代理配置

所有外部资源（图片、PDF）的获取都通过 HTTP 代理：

```go
const httpProxyAddr = "http://127.0.0.1:7890"
```

这允许在需要翻墙的环境中访问受限资源。

### 格式转换流程

#### OpenAI → Gemini (请求)

```
OpenAI 格式:
{
  "messages": [{
    "content": [
      {"type": "text", "text": "..."},
      {"type": "image_url", "image_url": {"url": "..."}}
    ]
  }]
}

↓ 转换

Gemini 格式:
{
  "contents": [{
    "parts": [
      {"text": "..."},
      {"inline_data": {"mime_type": "image/jpeg", "data": "base64..."}}
    ]
  }]
}
```

#### Gemini → OpenAI (响应)

响应格式已经是 OpenAI 兼容的，只需要处理：
- 添加 `tool_calls[].index` 字段
- 转换 `<thought>` 标签为 `reasoning_content`
- 移除多余的 `choices[0].index` 字段

### 安全措施

1. **文件大小限制**: 50MB 硬限制，防止 DoS 攻击
2. **格式验证**:
   - 图片: 使用 `image.DecodeConfig` 验证
   - PDF: 检查 `%PDF` 魔术字节
3. **MIME 类型验证**: 根据实际内容确定 MIME 类型
4. **URL 验证**: 防止无效 URL 攻击
5. **超时保护**: 30 秒获取超时，600 秒请求超时

### 性能优化

根据基准测试：

- **extractDataURL**: 42 ns/op - 极快
- **convertMultimodalContent**: 62 μs/op - 主要时间花在网络请求上

**优化建议**:
1. 添加图片 URL 缓存层
2. 对相同 URL 使用缓存的 base64 数据
3. 使用连接池优化 HTTP 请求
4. 考虑并行获取多张图片

## 错误处理

### 常见错误及解决方案

#### 1. 代理连接失败
```
Error: fetch failed: dial tcp 127.0.0.1:7890: connect: connection refused
```
**解决**: 启动 VPN/代理服务

#### 2. 文件过大
```
Error: file too large: 52428801 bytes
```
**解决**: 文件必须小于 50MB

#### 3. 无效图片格式
```
Error: invalid image: image: unknown format
```
**解决**: 确保是 JPEG/PNG/GIF 格式

#### 4. 无效 PDF
```
Error: not a valid PDF file
```
**解决**: 确保文件以 `%PDF` 开头

#### 5. 网络超时
```
Error: context deadline exceeded
```
**解决**:
- 检查网络连接
- 检查代理配置
- 尝试更小的文件

## 兼容性

### OpenAI SDK

完全兼容 OpenAI 官方 SDK：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:5345/v1",
    api_key="dummy"  # 认证在服务器端处理
)

# Vision API
response = client.chat.completions.create(
    model="gemini-2.0-flash-exp",
    messages=[
        {
            "role": "user",
            "content": [
                {"type": "text", "text": "What's in this image?"},
                {
                    "type": "image_url",
                    "image_url": {"url": "https://example.com/image.jpg"}
                }
            ]
        }
    ]
)

# Image Generation
image = client.images.generate(
    prompt="A cute cat",
    n=1,
    size="1024x1024"
)
```

### LangChain

```python
from langchain_openai import ChatOpenAI
from langchain.schema.messages import HumanMessage

llm = ChatOpenAI(
    base_url="http://localhost:5345/v1",
    model="gemini-2.0-flash-exp"
)

message = HumanMessage(
    content=[
        {"type": "text", "text": "Describe this image"},
        {
            "type": "image_url",
            "image_url": {"url": "https://example.com/image.jpg"}
        }
    ]
)

response = llm.invoke([message])
```

## 测试

### 单元测试

```bash
# 运行所有测试
go test -v

# 运行特定测试
go test -v -run TestFetchImage
go test -v -run TestPDF
go test -v -run TestImageGeneration

# 生成覆盖率报告
go test -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### 集成测试

```bash
# 测试 Vision API
./examples/test_vision.sh

# 测试 PDF 分析
./examples/test_pdf.sh

# 测试图片生成
./examples/test_image_generation.sh
```

详细测试说明请参考 [TESTING.md](TESTING.md)。

## 限制和注意事项

### 当前限制

1. **文件大小**: 最大 50MB
2. **超时时间**:
   - 文件获取: 30 秒
   - 完整请求: 600 秒 (10 分钟)
3. **并发**: 受 HTTP 客户端连接池限制
4. **代理**: 硬编码为 127.0.0.1:7890

### 未来改进

- [ ] 支持可配置的代理地址（环境变量）
- [ ] 添加图片缓存层
- [ ] 支持更大文件（分块上传）
- [ ] 支持视频输入
- [ ] 支持音频输入
- [ ] 添加速率限制
- [ ] 添加使用统计和监控
- [ ] 支持图片编辑（DALL-E edit API）
- [ ] 支持图片变体（DALL-E variations API）

## 配置

### 环境变量（计划支持）

```bash
# HTTP 代理配置
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890

# 文件大小限制 (字节)
export MAX_FILE_SIZE=52428800  # 50MB

# 超时配置 (秒)
export FETCH_TIMEOUT=30
export REQUEST_TIMEOUT=600
```

### 代码配置

当前配置在 `main.go` 中：

```go
const (
    httpProxyAddr = "http://127.0.0.1:7890"  // 代理地址
    maxFileSize   = 50 * 1024 * 1024         // 50MB
)
```

## API 端点总结

| 端点 | 方法 | 功能 | OpenAI 兼容 |
|------|------|------|-------------|
| `/v1/chat/completions` | POST | 聊天完成（支持多模态） | ✅ |
| `/v1/images/generations` | POST | 图片生成 | ✅ |
| `/v1/ws` | WebSocket | WebSocket 隧道 | - |

## 故障排查

### 日志级别

服务器会记录详细的操作日志：

```
2026/01/09 23:15:19 Successfully injected extra_body.google.thinking_config
2026/01/09 23:15:19 Failed to fetch image https://...: HTTP 404
2026/01/09 23:15:20 [UserID: user-1] Injected extra_content for tool_call_id: call_abc123
```

### 调试技巧

1. **检查代理日志**: 在 VPN/代理软件中查看连接日志
2. **网络抓包**: 使用 Charles 或 mitmproxy
3. **测试连接**:
   ```bash
   # 测试代理
   curl -x http://127.0.0.1:7890 https://www.google.com

   # 测试服务器
   curl http://localhost:5345/v1/chat/completions
   ```

### 常见问题

**Q: 图片获取很慢？**
A: 检查代理配置，或者使用 data URL 直接发送 base64 数据。

**Q: 支持哪些图片格式？**
A: JPEG, PNG, GIF。其他格式会被拒绝。

**Q: 可以同时发送多张图片吗？**
A: 可以，在 `content` 数组中添加多个 `image_url` 对象。

**Q: PDF 必须在线吗？**
A: 不，可以使用 data URL 发送 base64 编码的 PDF。

**Q: 图片生成需要多久？**
A: 取决于 Gemini Imagen API 的响应时间，通常 5-30 秒。

## 更多资源

- [主文档](CLAUDE.md) - 完整代码分析
- [API 参考](API_REFERENCE.md) - OpenAI 和 Gemini API 对比
- [测试文档](TESTING.md) - 详细测试指南
- [示例脚本](examples/) - 可执行的测试脚本

## 贡献

欢迎提交问题和 Pull Request！

主要功能贡献者: Claude Sonnet 4.5 🤖
