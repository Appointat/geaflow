#!/bin/bash

# 测试 Vision API - 图片分析示例
# 确保服务器正在运行: ./wsproxy

BASE_URL="http://localhost:5345"

echo "=== 测试 1: 使用在线图片 URL ==="
curl -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "这张图片里有什么？请详细描述。"},
        {
          "type": "image_url",
          "image_url": {
            "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/3/3a/Cat03.jpg/1200px-Cat03.jpg"
          }
        }
      ]
    }],
    "stream": false
  }' | jq .

echo -e "\n\n=== 测试 2: 使用 data URL (base64 编码) ==="

# 创建一个小的测试图片 (1x1 红色像素 PNG)
DATA_URL="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="

curl -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"gemini-2.0-flash-exp\",
    \"messages\": [{
      \"role\": \"user\",
      \"content\": [
        {\"type\": \"text\", \"text\": \"这是什么颜色的图片？\"},
        {
          \"type\": \"image_url\",
          \"image_url\": {
            \"url\": \"$DATA_URL\"
          }
        }
      ]
    }],
    \"stream\": false
  }" | jq .

echo -e "\n\n=== 测试 3: 多张图片分析 ==="
curl -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "比较这两张图片的异同："},
        {
          "type": "image_url",
          "image_url": {"url": "https://picsum.photos/200/300"}
        },
        {
          "type": "image_url",
          "image_url": {"url": "https://picsum.photos/300/200"}
        }
      ]
    }],
    "stream": false
  }' | jq .

echo -e "\n\n测试完成！"
echo "注意："
echo "1. 确保服务器正在运行: ./wsproxy"
echo "2. 确保 WebSocket 客户端已连接"
echo "3. 确保代理配置正确 (http://127.0.0.1:7890)"
echo "4. 如果看到错误，检查服务器日志"
