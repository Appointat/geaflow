#!/bin/bash

# 测试图片生成功能 (DALL-E 风格 API)
# 确保服务器正在运行: ./wsproxy

BASE_URL="http://localhost:5345"

echo "=== 测试 1: 生成图片 - URL 格式 ==="
curl -X POST "$BASE_URL/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A cute cat playing piano in a cozy room, digital art style",
    "model": "imagen-3.0",
    "n": 1,
    "size": "1024x1024",
    "response_format": "url"
  }' | jq .

echo -e "\n\n=== 测试 2: 生成图片 - Base64 JSON 格式 ==="
curl -X POST "$BASE_URL/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A beautiful sunset over mountains with vibrant colors",
    "n": 1,
    "size": "1024x1024",
    "response_format": "b64_json"
  }' | jq '.data[0].b64_json' | head -c 100

echo -e "\n\n=== 测试 3: 生成多张图片 ==="
curl -X POST "$BASE_URL/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Abstract geometric patterns in pastel colors",
    "n": 2,
    "size": "1024x1024",
    "response_format": "url"
  }' | jq '.data | length'

echo -e "\n\n=== 测试 4: 不同尺寸 - 16:9 ==="
curl -X POST "$BASE_URL/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A wide landscape with rolling hills and blue sky",
    "n": 1,
    "size": "1792x1024",
    "response_format": "url"
  }' | jq .

echo -e "\n\n=== 测试 5: 不同尺寸 - 9:16 (竖屏) ==="
curl -X POST "$BASE_URL/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A tall waterfall in a forest, portrait orientation",
    "n": 1,
    "size": "1024x1792",
    "response_format": "url"
  }' | jq .

echo -e "\n\n=== 测试 6: 保存生成的图片 ==="
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A friendly robot with glowing eyes",
    "n": 1,
    "size": "512x512",
    "response_format": "b64_json"
  }')

# 提取 base64 数据并保存为文件
B64_DATA=$(echo "$RESPONSE" | jq -r '.data[0].b64_json')

if [ ! -z "$B64_DATA" ] && [ "$B64_DATA" != "null" ]; then
    echo "$B64_DATA" | base64 -d > generated_image.png
    echo "图片已保存到: generated_image.png"
    file generated_image.png
else
    echo "未能获取图片数据"
fi

echo -e "\n\n测试完成！"
echo "注意："
echo "1. response_format: 'url' 返回 data URL (可直接在浏览器中查看)"
echo "2. response_format: 'b64_json' 返回纯 base64 数据"
echo "3. 支持的尺寸:"
echo "   - 1024x1024 (1:1)"
echo "   - 1792x1024 (16:9)"
echo "   - 1024x1792 (9:16)"
echo "   - 512x512 (1:1)"
echo "   - 256x256 (1:1)"
