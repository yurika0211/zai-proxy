package filter

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"zai-proxy/internal/model"
)

// altToolCallPattern 匹配 [TOOL]...[/TOOL] 和 [TOOL_CALL]...[/TOOL_CALL] 格式
var altToolCallPattern = regexp.MustCompile(`\[TOOL(?:_CALL)?\]\s*([\s\S]*?)\s*\[/TOOL(?:_CALL)?\]`)

// jsonBlockPattern 匹配 markdown JSON 代码块中的 tool call
var jsonBlockPattern = regexp.MustCompile("```json\\s*\\n(\\{[\\s\\S]*?\"name\"[\\s\\S]*?\\})\\s*\\n```")

// diamondToolCallPattern 匹配 ◇name◇args◇ 格式（prompt-injected tool calls）
var diamondToolCallPattern = regexp.MustCompile(`◇([a-zA-Z0-9_]+)◇(.*?)◇`)

// ExtractPromptToolCalls 从文本中提取所有 tool call 块（支持多种格式），
// 返回清理后的文本和解析出的 tool calls。
func ExtractPromptToolCalls(content string) (cleanContent string, toolCalls []model.ToolCall) {
	var allCalls []model.ToolCall
	cleaned := content

	// 首先尝试 <tool_call> 格式（使用 brace-counting 处理嵌套 JSON）
	if result, calls := extractToolCallTags(cleaned); len(calls) > 0 {
		allCalls = append(allCalls, calls...)
		cleaned = result
	}

	// 然后尝试 [TOOL]/[TOOL_CALL] 和 markdown JSON 格式
	for _, pattern := range []*regexp.Regexp{altToolCallPattern, jsonBlockPattern} {
		matches := pattern.FindAllStringSubmatchIndex(cleaned, -1)
		if len(matches) == 0 {
			continue
		}
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			fullStart, fullEnd := match[0], match[1]
			groupStart, groupEnd := match[2], match[3]
			jsonStr := cleaned[groupStart:groupEnd]
			if calls := parsePromptToolCallJSON(jsonStr); len(calls) > 0 {
				allCalls = append(allCalls, calls...)
			}
			cleaned = cleaned[:fullStart] + cleaned[fullEnd:]
		}
	}

	// 最后尝试 ◇name◇args◇ 格式（prompt-injected tool calls）
	matches := diamondToolCallPattern.FindAllStringSubmatchIndex(cleaned, -1)
	if len(matches) > 0 {
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			fullStart, fullEnd := match[0], match[1]
			nameStart, nameEnd := match[2], match[3]
			argsStart, argsEnd := match[4], match[5]

			name := cleaned[nameStart:nameEnd]
			args := cleaned[argsStart:argsEnd]

			allCalls = append(allCalls, model.ToolCall{
				Function: model.FunctionCall{
					Name:      name,
					Arguments: args,
				},
			})
			cleaned = cleaned[:fullStart] + cleaned[fullEnd:]
		}
	}

	if len(allCalls) == 0 {
		return content, nil
	}

	// 清理多余空行
	cleaned = strings.TrimSpace(cleaned)
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}

	// 为每个 tool call 分配 ID
	for i := range allCalls {
		if allCalls[i].ID == "" {
			allCalls[i].ID = fmt.Sprintf("call_%s", uuid.New().String()[:24])
		}
		allCalls[i].Index = i
		allCalls[i].Type = "function"
	}

	return cleaned, allCalls
}

// extractToolCallTags 使用 brace-counting 提取 <tool_call> 块中的 JSON，
// 正确处理嵌套 JSON 对象和缺失/错误的闭合标签。
func extractToolCallTags(content string) (cleanContent string, toolCalls []model.ToolCall) {
	const openTag = "<tool_call>"
	var calls []model.ToolCall
	cleaned := content

	// 从后向前查找所有 <tool_call> 标记，避免索引偏移
	var tagPositions []int
	searchFrom := 0
	for {
		idx := strings.Index(cleaned[searchFrom:], openTag)
		if idx == -1 {
			break
		}
		tagPositions = append(tagPositions, searchFrom+idx)
		searchFrom += idx + len(openTag)
	}

	// 从后向前处理
	for i := len(tagPositions) - 1; i >= 0; i-- {
		tagStart := tagPositions[i]
		afterTag := cleaned[tagStart+len(openTag):]

		// 找到 JSON 对象的起始 {
		jsonStart := strings.Index(afterTag, "{")
		if jsonStart == -1 {
			continue
		}

		// 提取 { 之前可能存在的函数名前缀（如 <tool_call>Read{...}）
		funcNamePrefix := strings.TrimSpace(afterTag[:jsonStart])

		// 使用 brace-counting 找到匹配的 }
		jsonEnd := findMatchingBrace(afterTag[jsonStart:])
		if jsonEnd == -1 {
			continue
		}

		jsonStr := afterTag[jsonStart : jsonStart+jsonEnd+1]
		parsed := parsePromptToolCallJSON(jsonStr)

		// 如果标准格式解析失败，但有函数名前缀，尝试当作 FuncName{args} 格式
		if len(parsed) == 0 && funcNamePrefix != "" {
			wrapped := fmt.Sprintf(`{"name": %q, "arguments": %s}`, funcNamePrefix, jsonStr)
			parsed = parsePromptToolCallJSON(wrapped)
		}

		if len(parsed) == 0 {
			continue
		}
		calls = append(parsed, calls...)

		// 计算要移除的范围：从 <tool_call> 到 JSON 结束 + 可选的闭合标签
		blockEnd := tagStart + len(openTag) + jsonStart + jsonEnd + 1
		remaining := cleaned[blockEnd:]
		// 移除可选的闭合标签
		for _, closeTag := range []string{"</tool_call>", "</think>"} {
			trimmed := strings.TrimLeft(remaining, " \t\n")
			if strings.HasPrefix(trimmed, closeTag) {
				blockEnd = blockEnd + (len(remaining) - len(trimmed)) + len(closeTag)
				break
			}
		}
		cleaned = cleaned[:tagStart] + cleaned[blockEnd:]
	}

	return cleaned, calls
}

// findMatchingBrace 在以 { 开头的字符串中找到匹配的 } 的索引。
// 返回 -1 如果未找到匹配的闭合大括号。
func findMatchingBrace(s string) int {
	if len(s) == 0 || s[0] != '{' {
		return -1
	}
	depth := 0
	inString := false
	escape := false
	for i, ch := range s {
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parsePromptToolCallJSON 解析 <tool_call> 内的 JSON
func parsePromptToolCallJSON(content string) []model.ToolCall {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	// 标准格式: {"name": "xxx", "arguments": {...}}
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Input     json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal([]byte(content), &call); err == nil && call.Name != "" {
		argsStr := decodePromptToolArguments(call.Arguments)
		if argsStr == "" {
			argsStr = decodePromptToolArguments(call.Input)
		}
		return []model.ToolCall{{
			Function: model.FunctionCall{
				Name:      call.Name,
				Arguments: argsStr,
			},
		}}
	}

	return nil
}

func decodePromptToolArguments(raw json.RawMessage) string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] != '"' {
		return string(raw)
	}
	var decoded string
	if err := json.Unmarshal(raw, &decoded); err == nil {
		return decoded
	}
	return string(raw)
}

// HasPromptToolCallOpen 检测文本中是否有未关闭的 tool call 标签
func HasPromptToolCallOpen(content string) bool {
	// <tool_call>
	if strings.Count(content, "<tool_call>") > strings.Count(content, "</tool_call>") {
		return true
	}
	// [TOOL] / [TOOL_CALL]
	if strings.Count(content, "[TOOL]") > strings.Count(content, "[/TOOL]") {
		return true
	}
	if strings.Count(content, "[TOOL_CALL]") > strings.Count(content, "[/TOOL_CALL]") {
		return true
	}
	return false
}
