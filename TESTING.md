# 测试文档

## 测试概览

本项目包含全面的单元测试、集成测试和基准测试，涵盖所有多模态功能。

## 测试统计

- **总测试数**: 17 个测试
- **测试覆盖率**: 30.3% (新功能覆盖率 60-100%)
- **所有测试**: ✅ 通过

### 覆盖率详情

新增的多模态功能测试覆盖率：

| 功能 | 覆盖率 |
|------|--------|
| `extractDataURL` | 100% |
| `mapSizeToAspectRatio` | 100% |
| `fetchImageToBase64` | 84.6% |
| `processContentArray` | 82.4% |
| `processImageGenerationResponse` | 78.9% |
| `processMultimodalPart` | 70.6% |
| `fetchPDFToBase64` | 63.6% |
| `convertMultimodalContent` | 60.0% |
| `handleImageGeneration` | 61.5% |

## 运行测试

### 运行所有测试

```bash
go test -v
```

输出示例：

```
=== RUN   TestExtractDataURL
--- PASS: TestExtractDataURL (0.00s)
=== RUN   TestFetchImageToBase64_DataURL
--- PASS: TestFetchImageToBase64_DataURL (0.00s)
...
PASS
ok      wsproxy 0.235s
```

### 运行特定测试

```bash
# 测试 data URL 提取
go test -v -run TestExtractDataURL

# 测试图片获取
go test -v -run TestFetchImage

# 测试 PDF 处理
go test -v -run TestFetchPDF

# 测试格式转换
go test -v -run TestConvert

# 测试图片生成
go test -v -run TestImageGeneration
```

### 生成覆盖率报告

```bash
# 生成覆盖率文件
go test -cover -coverprofile=coverage.out

# 查看覆盖率统计
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
# 用浏览器打开 coverage.html
```

### 运行基准测试

```bash
go test -bench=. -benchmem
```

输出示例：

```
BenchmarkExtractDataURL-14              26916944        42.14 ns/op      64 B/op       2 allocs/op
BenchmarkConvertMultimodalContent-14       19786     61989 ns/op   14221 B/op     132 allocs/op
```

## 测试分类

### 1. 单元测试

#### Data URL 解析测试

- `TestExtractDataURL`: 测试 data URL 格式解析
  - 有效的 JPEG data URL
  - 有效的 PNG data URL
  - 默认 MIME 类型处理
  - 无效 data URL 错误处理

#### 图片获取测试

- `TestFetchImageToBase64_DataURL`: 测试 data URL 图片处理
- `TestFetchImageToBase64_HTTPUrl`: 测试 HTTP URL 图片获取
- `TestFetchImageToBase64_SizeLimit`: 测试文件大小限制
- `TestFetchImageToBase64_InvalidURL`: 测试无效 URL 处理
- `TestFetchImageToBase64_404`: 测试 404 错误处理
- `TestFetchImageToBase64_InvalidImage`: 测试无效图片数据处理

#### PDF 处理测试

- `TestFetchPDFToBase64`: 测试 PDF 获取和编码
- `TestFetchPDFToBase64_InvalidPDF`: 测试无效 PDF 检测

#### 格式转换测试

- `TestProcessContentArray_ImageURL`: 测试 OpenAI 图片格式转换
- `TestProcessContentArray_StringContent`: 测试纯文本内容处理
- `TestProcessContentArray_PDFDocument`: 测试 PDF 文档转换
- `TestProcessMultimodalPart_FileData`: 测试 Gemini file_data 转换
- `TestProcessMultimodalPart_AlreadyInlineData`: 测试已有 inline_data 处理
- `TestConvertMultimodalContent_OpenAIFormat`: 测试完整 OpenAI 格式转换
- `TestConvertMultimodalContent_InvalidJSON`: 测试无效 JSON 处理

#### 图片生成测试

- `TestMapSizeToAspectRatio`: 测试 DALL-E 尺寸到 Imagen 宽高比映射

### 2. 集成测试

- `TestHandleImageGeneration_Integration`: 测试完整的图片生成流程
  - WebSocket 连接
  - 请求转发
  - 响应转换
  - DALL-E 格式输出

### 3. 基准测试

- `BenchmarkExtractDataURL`: data URL 解析性能测试
  - **性能**: 42.14 ns/op
  - **内存**: 64 B/op, 2 allocs/op

- `BenchmarkConvertMultimodalContent`: 多模态转换性能测试
  - **性能**: 61989 ns/op (~62 μs)
  - **内存**: 14221 B/op, 132 allocs/op

## 功能测试

### 测试图片输入 (Vision)

```bash
# 启动服务器
./wsproxy

# 在另一个终端测试（需要 WebSocket 客户端连接）
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "这张图片里有什么？"},
        {"type": "image_url", "image_url": {"url": "https://example.com/image.jpg"}}
      ]
    }]
  }'
```

### 测试 data URL 图片

```bash
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "描述这张图片"},
        {"type": "image_url", "image_url": {"url": "data:image/png;base64,iVBORw0KG..."}}
      ]
    }]
  }'
```

### 测试 PDF 输入

```bash
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "总结这份文档"},
        {"type": "pdf", "document_url": "https://example.com/document.pdf"}
      ]
    }]
  }'
```

### 测试图片生成

```bash
curl -X POST http://localhost:5345/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "一只可爱的猫在弹钢琴",
    "model": "imagen-3.0",
    "n": 1,
    "size": "1024x1024",
    "response_format": "url"
  }'
```

### 测试 base64 响应格式

```bash
curl -X POST http://localhost:5345/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A beautiful sunset over mountains",
    "n": 1,
    "size": "1024x1024",
    "response_format": "b64_json"
  }'
```

## 代理配置测试

### 验证代理是否工作

```bash
# 设置代理环境变量（可选，代码中已硬编码）
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890

# 测试通过代理获取图片
curl -X POST http://localhost:5345/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "Describe this"},
        {"type": "image_url", "image_url": {"url": "https://www.google.com/images/branding/googlelogo/1x/googlelogo_color_272x92dp.png"}}
      ]
    }]
  }'
```

### 检查代理日志

如果你的代理（如 Clash 或 v2ray）有日志功能，你应该能看到：

- 来自 `wsproxy` 的连接
- 到目标图片 URL 的请求

## 测试数据生成

测试文件中包含辅助函数来生成测试数据：

### 生成测试图片

```go
// 创建 10x10 像素的红色 PNG 图片
imgBytes := createTestImage(10, 10)
```

### 生成测试 PDF

```go
// 创建最小的有效 PDF 文件
pdfBytes := createTestPDF()
```

## 错误场景测试

所有测试都包含错误处理验证：

1. **无效 URL**: 测试无效的 URL 格式
2. **HTTP 错误**: 测试 404、500 等错误响应
3. **文件大小限制**: 测试超过 50MB 的文件
4. **无效格式**: 测试非图片/PDF 数据
5. **网络超时**: 通过 httptest 模拟超时
6. **无效 JSON**: 测试损坏的请求数据

## 持续集成

### GitHub Actions 配置示例

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out

      - name: Generate coverage report
        run: go tool cover -html=coverage.out -o coverage.html

      - name: Upload coverage
        uses: actions/upload-artifact@v3
        with:
          name: coverage-report
          path: coverage.html
```

## 性能优化建议

根据基准测试结果：

1. **extractDataURL** (42 ns/op): 性能优秀，无需优化
2. **convertMultimodalContent** (62 μs/op):
   - 大部分时间花在 HTTP 请求上
   - 可以考虑添加图片缓存层
   - 对于相同 URL 的重复请求使用缓存

## 故障排查

### 测试失败：图片获取超时

```
Error: fetch failed: context deadline exceeded
```

**解决方案**:

- 检查代理配置 (127.0.0.1:7890)
- 确保代理服务正在运行
- 检查防火墙设置

### 测试失败：无效的图片格式

```
Error: invalid image: image: unknown format
```

**解决方案**:

- 确保导入了图片格式包：

  ```go
  _ "image/jpeg"
  _ "image/png"
  _ "image/gif"
  ```

### 测试失败：连接被拒绝

```
Error: dial tcp 127.0.0.1:7890: connect: connection refused
```

**解决方案**:

- 启动 VPN/代理服务
- 或者在测试中使用 mock HTTP 服务器

## 下一步

1. ✅ 添加更多边缘情况测试
2. ✅ 实现性能基准测试
3. ⏭️ 添加端到端测试（需要实际的 WebSocket 客户端）
4. ⏭️ 添加负载测试
5. ⏭️ 测试并发场景

## 贡献指南

添加新测试时：

1. 遵循现有测试命名约定：`Test<功能名>_<场景>`
2. 使用表驱动测试进行多个测试用例
3. 包含成功和失败场景
4. 添加适当的错误消息验证
5. 使用 `httptest` 模拟外部依赖
6. 更新此文档

## 测试最佳实践

1. **隔离性**: 每个测试独立运行，不依赖其他测试
2. **可重复性**: 测试结果稳定，不受外部因素影响
3. **快速**: 单元测试应该在毫秒级完成
4. **清晰**: 测试失败时，错误消息清楚指出问题
5. **完整**: 覆盖正常路径和错误路径

## 测试环境要求

- Go 1.22+
- gorilla/websocket 包
- 无需外部依赖（测试使用 httptest mock）
- 可选：运行在 127.0.0.1:7890 的代理服务（用于手动测试）
