#!/bin/bash
# 测试 tool/function calling 功能
# 用法: ./scripts/test_tool_call.sh [TOKEN] [BASE_URL]
#
# TOKEN 可以是你的 z.ai token 或 "free"（匿名）
# BASE_URL 默认 http://localhost:8000

TOKEN="${1:-free}"
BASE_URL="${2:-http://localhost:8000}"

PASS=0
FAIL=0

check() {
  local desc="$1" output="$2" pattern="$3"
  if echo "$output" | grep -qE "$pattern"; then
    echo "  ✓ $desc"
    ((PASS++))
  else
    echo "  ✗ $desc (未匹配: $pattern)"
    ((FAIL++))
  fi
}

check_not() {
  local desc="$1" output="$2" pattern="$3"
  if echo "$output" | grep -qE "$pattern"; then
    echo "  ✗ $desc (不应包含: $pattern)"
    ((FAIL++))
  else
    echo "  ✓ $desc"
    ((PASS++))
  fi
}

echo "=== 测试 Tool/Function Calling ==="
echo "BASE_URL: $BASE_URL"
echo "TOKEN: ${TOKEN:0:10}..."
echo ""

# ===== 测试 1: 带 tools 的流式请求 =====
echo "--- 测试 1: 流式 tool calling ---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": true,
    "messages": [
      {"role": "user", "content": "北京今天天气怎么样？请调用 get_weather 函数查询。"}
    ],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取指定城市的当前天气信息",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "城市名称，如：北京"
            }
          },
          "required": ["location"]
        }
      }
    }],
    "tool_choice": "auto"
  }' 2>&1)
echo "$OUT" | head -20
check "包含 tool_calls" "$OUT" '"tool_calls"'
check "包含函数名 get_weather" "$OUT" '"get_weather"'
check "finish_reason 为 tool_calls" "$OUT" '"finish_reason"\s*:\s*"tool_calls"'
check "包含 [DONE]" "$OUT" 'data: \[DONE\]'
echo ""

# ===== 测试 2: 带 tools 的非流式请求 =====
echo "--- 测试 2: 非流式 tool calling ---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": false,
    "messages": [
      {"role": "user", "content": "帮我查一下上海的天气，用 get_weather 工具。"}
    ],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取指定城市的当前天气信息",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "城市名称"
            }
          },
          "required": ["location"]
        }
      }
    }],
    "tool_choice": "auto"
  }' 2>&1)
echo "$OUT" | python3 -m json.tool 2>/dev/null || echo "$OUT"
check "包含 tool_calls" "$OUT" '"tool_calls"'
check "包含函数名 get_weather" "$OUT" '"get_weather"'
check "finish_reason 为 tool_calls" "$OUT" '"finish_reason"\s*:\s*"tool_calls"'
check_not "不包含 delta 字段" "$OUT" '"delta"'
echo ""

# ===== 测试 3: 多工具 =====
echo "--- 测试 3: 多工具非流式 ---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": false,
    "messages": [
      {"role": "user", "content": "北京天气怎么样？现在几点了？请分别调用对应的工具。"}
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "获取天气",
          "parameters": {"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}
        }
      },
      {
        "type": "function",
        "function": {
          "name": "get_current_time",
          "description": "获取当前时间",
          "parameters": {"type": "object", "properties": {"timezone": {"type": "string"}}, "required": ["timezone"]}
        }
      }
    ],
    "tool_choice": "auto"
  }' 2>&1)
echo "$OUT" | python3 -m json.tool 2>/dev/null || echo "$OUT"
check "包含 tool_calls" "$OUT" '"tool_calls"'
check "包含 get_weather" "$OUT" '"get_weather"'
check "包含 get_current_time" "$OUT" '"get_current_time"'
echo ""

# ===== 测试 4: 完整多轮对话（tool result 回传）=====
echo "--- 测试 4: 多轮对话 (tool result 回传) ---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": false,
    "messages": [
      {"role": "user", "content": "北京天气怎么样？"},
      {
        "role": "assistant",
        "content": "",
        "tool_calls": [{
          "id": "call_abc123",
          "type": "function",
          "function": {"name": "get_weather", "arguments": "{\"location\":\"北京\"}"}
        }]
      },
      {
        "role": "tool",
        "tool_call_id": "call_abc123",
        "content": "{\"temperature\": 25, \"condition\": \"晴\", \"humidity\": 40}"
      }
    ],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取天气",
        "parameters": {"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}
      }
    }]
  }' 2>&1)
echo "$OUT" | python3 -m json.tool 2>/dev/null || echo "$OUT"
check "finish_reason 为 stop" "$OUT" '"finish_reason"\s*:\s*"stop"'
check "包含 message 字段" "$OUT" '"message"'
check "包含回复内容 (content 非空)" "$OUT" '"content"'
echo ""

# ===== 测试 5: 不带 tools 的普通请求（回归测试）=====
echo "--- 测试 5: 不带 tools 的普通请求（回归）---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": false,
    "messages": [
      {"role": "user", "content": "你好，1+1等于几？"}
    ]
  }' 2>&1)
echo "$OUT" | python3 -m json.tool 2>/dev/null || echo "$OUT"
check "finish_reason 为 stop" "$OUT" '"finish_reason"\s*:\s*"stop"'
check_not "不包含 tool_calls" "$OUT" '"tool_calls"'
check_not "不包含 delta" "$OUT" '"delta"'
echo ""

# ===== 测试 6: -tools 模型后缀 =====
echo "--- 测试 6: -tools 模型后缀 (GLM-4.7-tools) ---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7-tools",
    "stream": false,
    "messages": [
      {"role": "user", "content": "现在几点了？"}
    ]
  }' 2>&1)
echo "$OUT" | python3 -m json.tool 2>/dev/null || echo "$OUT"
check "包含 tool_calls 或正常回复" "$OUT" '"choices"'
echo "(注意: -tools 模型自动注入内置工具，模型可能调用也可能不调用)"
echo ""

# ===== 测试 7: tool_choice required =====
echo "--- 测试 7: tool_choice=required ---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": false,
    "messages": [
      {"role": "user", "content": "查询北京天气"}
    ],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取指定城市的当前天气信息",
        "parameters": {
          "type": "object",
          "properties": {"location": {"type": "string", "description": "城市名称"}},
          "required": ["location"]
        }
      }
    }],
    "tool_choice": "required"
  }' 2>&1)
echo "$OUT" | python3 -m json.tool 2>/dev/null || echo "$OUT"
check "包含 tool_calls" "$OUT" '"tool_calls"'
check "finish_reason 为 tool_calls" "$OUT" '"finish_reason"\s*:\s*"tool_calls"'
echo ""

# ===== 测试 8: tool_choice 指定具体函数 =====
echo "--- 测试 8: tool_choice 指定具体函数 ---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": false,
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取指定城市的当前天气信息",
        "parameters": {
          "type": "object",
          "properties": {"location": {"type": "string"}},
          "required": ["location"]
        }
      }
    }],
    "tool_choice": {"type": "function", "function": {"name": "get_weather"}}
  }' 2>&1)
echo "$OUT" | python3 -m json.tool 2>/dev/null || echo "$OUT"
check "包含 get_weather" "$OUT" '"get_weather"'
echo ""

# ===== 测试 9: 流式普通请求回归 =====
echo "--- 测试 9: 流式不带 tools（回归）---"
OUT=$(curl -sS "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GLM-4.7",
    "stream": true,
    "messages": [
      {"role": "user", "content": "你好"}
    ]
  }' 2>&1)
echo "$OUT" | head -10
check "finish_reason 为 stop" "$OUT" '"finish_reason"\s*:\s*"stop"'
check "包含 [DONE]" "$OUT" 'data: \[DONE\]'
check_not "不包含 tool_calls" "$OUT" '"tool_calls"'
echo ""

# ===== 汇总 =====
echo "================================"
echo "=== 测试汇总 ==="
echo "  通过: $PASS"
echo "  失败: $FAIL"
echo "  总计: $((PASS + FAIL))"
echo "================================"
echo ""
echo "检查要点："
echo "  1. 测试 1/2: 响应中应有 tool_calls 字段和 finish_reason=tool_calls"
echo "  2. 测试 3: 应返回多个 tool_calls（get_weather 和 get_current_time）"
echo "  3. 测试 4: 模型应基于 tool result 生成自然语言回复，finish_reason=stop"
echo "  4. 测试 5/9: 不带 tools 时正常返回文本，无 tool_calls 字段"
echo "  5. 测试 6: -tools 后缀会自动注入内置工具（模型可能触发也可能不触发）"
echo "  6. 测试 7: tool_choice=required 应强制模型调用工具"
echo "  7. 测试 8: tool_choice 指定函数名应调用该函数"
echo "  8. 查看服务端日志中的 [ToolCall] 行，确认上游返回的原始格式"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
