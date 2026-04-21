package thinking

import (
	"strconv"
	"strings"
)

/**
 * validThinkingSuffixes 存储所有有效的思考级别后缀
 * 用于快速判断尾段是否为思考配置
 */
var validThinkingSuffixes = map[string]bool{
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
	"xhigh":   true,
	"max":     true,
	"none":    true,
	"auto":    true,
}

/**
 * knownAmbiguousModels 以思考关键字结尾的完整基础模型名白名单
 * 这些模型名的尾段虽然与思考后缀同名，但实际是模型名的一部分
 * 仅精确匹配整个模型名，不使用模式匹配，避免误伤其他模型的后缀解析
 */
var knownAmbiguousModels = map[string]bool{
	"gpt-5.1-codex-max": true,
}

/**
 * ParseModelSuffix 从模型名尾部逆向解析后缀
 * 不依赖模型白名单，纯粹匹配已知后缀关键字
 * 任何未识别的模型名都直接保留并转发给上游
 *
 * @param model - 原始模型名（可能包含思考后缀和/或 -fast 后缀）
 * @returns ParseResult - 解析结果
 */
func ParseModelSuffix(model string) ParseResult {
	model = strings.TrimSpace(model)
	if model == "" {
		return ParseResult{ModelName: model}
	}

	result := ParseResult{}

	/*
	 * 第一步：从右侧剥离 -fast（服务层级）
	 */
	lower := strings.ToLower(model)
	if strings.HasSuffix(lower, "-fast") && len(model) > 5 {
		result.IsFast = true
		result.ServiceTier = "priority" /* 与 ApplyThinking 写入上游的 service_tier 一致 */
		model = model[:len(model)-5]
	}

	/*
	 * 第二步：从右侧剥离思考后缀（级别名或数字预算）
	 * 找到最后一个连字符，检查尾段是否为已知思考后缀
	 * 先排除已知的歧义模型名（如 codex-max、codex-auto）
	 */
	lastDash := strings.LastIndex(model, "-")
	if lastDash > 0 && lastDash < len(model)-1 {
		tail := strings.ToLower(model[lastDash+1:])

		/* 检查完整模型名是否在歧义白名单中（精确匹配） */
		isAmbiguous := knownAmbiguousModels[strings.ToLower(model)]

		if !isAmbiguous {
			if validThinkingSuffixes[tail] {
				/* 匹配到思考级别后缀 */
				result.HasSuffix = true
				result.RawSuffix = tail
				model = model[:lastDash]
			} else if v, err := strconv.Atoi(tail); err == nil && v > 100 {
				/*
				 * 匹配到数字 token 预算（必须 > 100）
				 * 版本号（如 5、4、3）不会超过 100，token 预算通常 > 512
				 * 避免 gpt-5 中的 "5" 被误判为预算
				 */
				result.HasSuffix = true
				result.RawSuffix = tail
				model = model[:lastDash]
			}
		}
	}

	/* 剩余部分即为真实模型名 */
	result.ModelName = model
	return result
}

/**
 * ParseSuffixToConfig 将原始后缀字符串转换为 ThinkingConfig
 *
 * 解析优先级：
 *   1. 特殊值：none → ModeNone, auto/-1 → ModeAuto
 *   2. 级别名：minimal/low/medium/high/xhigh/max → ModeLevel
 *   3. 数字值：正整数 → ModeBudget, 0 → ModeNone
 *
 * @param rawSuffix - 原始后缀字符串
 * @returns ThinkingConfig - 解析后的思考配置
 */
func ParseSuffixToConfig(rawSuffix string) ThinkingConfig {
	rawSuffix = strings.TrimSpace(strings.ToLower(rawSuffix))
	if rawSuffix == "" {
		return ThinkingConfig{}
	}

	/* 1. 特殊值 */
	switch rawSuffix {
	case "none":
		return ThinkingConfig{Mode: ModeNone, Budget: 0}
	case "auto", "-1":
		return ThinkingConfig{Mode: ModeAuto, Budget: -1}
	}

	/* 2. 级别名 */
	switch rawSuffix {
	case "minimal":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelMinimal}
	case "low":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelLow}
	case "medium":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelMedium}
	case "high":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelHigh}
	case "xhigh":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelXHigh}
	case "max":
		return ThinkingConfig{Mode: ModeLevel, Level: LevelMax}
	}

	/* 3. 数字值 */
	if value, err := strconv.Atoi(rawSuffix); err == nil {
		if value == 0 {
			return ThinkingConfig{Mode: ModeNone, Budget: 0}
		}
		if value > 0 {
			return ThinkingConfig{Mode: ModeBudget, Budget: value}
		}
	}

	return ThinkingConfig{}
}
