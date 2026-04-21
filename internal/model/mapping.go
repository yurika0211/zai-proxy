package model

import "strings"

// 基础模型映射（不包含标签后缀）
var BaseModelMapping = map[string]string{
	"GLM-4.5":      "0727-360B-API",
	"GLM-4.6":      "GLM-4-6-API-V1",
	"GLM-4.7":      "glm-4.7",
	"GLM-4.5-V":    "glm-4.5v",
	"GLM-4.6-V":    "glm-4.6v",
	"GLM-4.5-Air":  "0727-106B-API",
	"0808-360B-DR": "0808-360B-DR",
}

// Claude 模型名到 GLM 基础模型名的映射
var ClaudeModelMapping = map[string]string{
	"claude-opus-4-6":            "GLM-4.7",
	"claude-opus-4-5-20250514":   "GLM-4.7",
	"claude-sonnet-4-6":          "GLM-4.7",
	"claude-sonnet-4-5-20241022": "GLM-4.7",
	"claude-haiku-4-5":           "GLM-4.5-Air",
	"claude-haiku-4-5-20251001":  "GLM-4.5-Air",
	"claude-3-5-sonnet-20241022": "GLM-4.7",
	"claude-3-5-haiku-20241022":  "GLM-4.5-Air",
}

// ResolveClaudeModel maps a Claude model name to a GLM model name with appropriate tags.
// Returns the resolved model name and whether thinking should be enabled.
func ResolveClaudeModel(model string, thinkingEnabled bool) (resolvedModel string, enableThinking bool) {
	base, ok := ClaudeModelMapping[model]
	if !ok {
		// Unknown model: fallback to GLM-4.7
		base = "GLM-4.7"
	}

	enableThinking = thinkingEnabled
	// Opus models always enable thinking
	if strings.Contains(model, "opus") {
		enableThinking = true
	}

	resolvedModel = base
	if enableThinking {
		resolvedModel += "-thinking"
	}
	return resolvedModel, enableThinking
}

// v1/models 返回的模型列表（不包含所有标签组合）
var ModelList = []string{
	"GLM-4.5",
	"GLM-4.6",
	"GLM-4.7",
	"GLM-4.7-thinking",
	"GLM-4.7-thinking-search",
	"GLM-4.7-tools",
	"GLM-4.7-tools-thinking",
	"GLM-4.5-V",
	"GLM-4.6-V",
	"GLM-4.6-V-thinking",
	"GLM-4.5-Air",
	// "0808-360B-DR",
}

// 解析模型名称，提取基础模型名和标签
// 支持 -thinking、-search 和 -tools 标签的任意排列组合
func ParseModelName(model string) (baseModel string, enableThinking bool, enableSearch bool, enableTools bool) {
	enableThinking = false
	enableSearch = false
	enableTools = false
	baseModel = model

	// 检查并移除 -thinking、-search 和 -tools 标签（任意顺序）
	for {
		if strings.HasSuffix(baseModel, "-thinking") {
			enableThinking = true
			baseModel = strings.TrimSuffix(baseModel, "-thinking")
		} else if strings.HasSuffix(baseModel, "-search") {
			enableSearch = true
			baseModel = strings.TrimSuffix(baseModel, "-search")
		} else if strings.HasSuffix(baseModel, "-tools") {
			enableTools = true
			baseModel = strings.TrimSuffix(baseModel, "-tools")
		} else {
			break
		}
	}

	return baseModel, enableThinking, enableSearch, enableTools
}

func IsThinkingModel(model string) bool {
	_, enableThinking, _, _ := ParseModelName(model)
	return enableThinking
}

func IsSearchModel(model string) bool {
	_, _, enableSearch, _ := ParseModelName(model)
	return enableSearch
}

func IsToolsModel(model string) bool {
	_, _, _, enableTools := ParseModelName(model)
	return enableTools
}

func GetTargetModel(model string) string {
	baseModel, _, _, _ := ParseModelName(model)
	if target, ok := BaseModelMapping[baseModel]; ok {
		return target
	}
	// Fallback: unknown models default to glm-4.7
	return "glm-4.7"
}
