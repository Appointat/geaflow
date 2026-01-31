#!/bin/bash

# 测试 PDF 分析功能
# 确保服务器正在运行: ./wsproxy

BASE_URL="http://localhost:5345"

echo "=== 测试 1: 分析在线 PDF 文档 ==="
curl -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "请总结这份 PDF 文档的主要内容："},
        {
          "type": "pdf",
          "document_url": "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf"
        }
      ]
    }],
    "stream": false
  }' | jq .

echo -e "\n\n=== 测试 2: 使用 data URL (base64 编码的 PDF) ==="

# 创建最小的 PDF (base64 编码)
PDF_BASE64=$(cat <<'EOF' | base64
%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
>>
endobj
4 0 obj
<<
/Length 44
>>
stream
BT
/F1 12 Tf
100 700 Td
(Hello World) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000214 00000 n
trailer
<<
/Size 5
/Root 1 0 R
>>
startxref
307
%%EOF
EOF
)

DATA_URL="data:application/pdf;base64,$PDF_BASE64"

curl -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"gemini-2.0-flash-exp\",
    \"messages\": [{
      \"role\": \"user\",
      \"content\": [
        {\"type\": \"text\", \"text\": \"这份 PDF 里写的是什么？\"},
        {
          \"type\": \"pdf\",
          \"document_url\": \"$DATA_URL\"
        }
      ]
    }],
    \"stream\": false
  }" | jq .

echo -e "\n\n=== 测试 3: PDF 和图片混合分析 ==="
curl -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "对比这份文档和这张图片："},
        {
          "type": "pdf",
          "document_url": "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf"
        },
        {
          "type": "image_url",
          "image_url": {"url": "https://picsum.photos/400/300"}
        }
      ]
    }],
    "stream": false
  }' | jq .

echo -e "\n\n测试完成！"
echo "注意："
echo "1. PDF 文件大小限制: 50MB"
echo "2. 必须是有效的 PDF 文件（以 %PDF 开头）"
echo "3. 支持在线 URL 和 data URL 两种格式"
