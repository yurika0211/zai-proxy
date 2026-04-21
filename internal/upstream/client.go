package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/corpix/uarand"
	"github.com/google/uuid"

	"zai-proxy/internal/auth"
	"zai-proxy/internal/logger"
	"zai-proxy/internal/model"
	builtintools "zai-proxy/internal/tools"
	"zai-proxy/internal/version"
)

func ExtractLatestUserContent(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			text, _ := messages[i].ParseContent()
			return text
		}
	}
	return ""
}

func ExtractAllImageURLs(messages []model.Message) []string {
	var allImageURLs []string
	for _, msg := range messages {
		_, imageURLs := msg.ParseContent()
		allImageURLs = append(allImageURLs, imageURLs...)
	}
	return allImageURLs
}

func MakeUpstreamRequest(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}, reqParams model.RequestParams) (*http.Response, string, error) {
	payload, err := auth.DecodeJWTPayload(token)
	if err != nil || payload == nil {
		return nil, "", fmt.Errorf("invalid token")
	}

	userID := payload.ID
	chatID := uuid.New().String()
	timestamp := time.Now().UnixMilli()
	requestID := uuid.New().String()
	userMsgID := uuid.New().String()

	targetModel := model.GetTargetModel(modelName)
	latestUserContent := ExtractLatestUserContent(messages)
	imageURLs := ExtractAllImageURLs(messages)

	signature := auth.GenerateSignature(userID, requestID, latestUserContent, timestamp)

	url := fmt.Sprintf("https://chat.z.ai/api/v2/chat/completions?timestamp=%d&requestId=%s&user_id=%s&version=0.0.1&platform=web&token=%s&current_url=%s&pathname=%s&signature_timestamp=%d",
		timestamp, requestID, userID, token,
		fmt.Sprintf("https://chat.z.ai/c/%s", chatID),
		fmt.Sprintf("/c/%s", chatID),
		timestamp)

	enableThinking := model.IsThinkingModel(modelName)
	autoWebSearch := model.IsSearchModel(modelName)
	if targetModel == "glm-4.5v" || targetModel == "glm-4.6v" {
		autoWebSearch = false
	}

	var mcpServers []string
	if targetModel == "glm-4.6v" {
		mcpServers = []string{"vlm-image-search", "vlm-image-recognition", "vlm-image-processing"}
	}

	urlToFileID := make(map[string]string)
	var filesData []map[string]interface{}
	if len(imageURLs) > 0 {
		files, _ := UploadImages(token, imageURLs)
		for i, f := range files {
			if i < len(imageURLs) {
				urlToFileID[imageURLs[i]] = f.ID
			}
			filesData = append(filesData, map[string]interface{}{
				"type":            f.Type,
				"file":            f.File,
				"id":              f.ID,
				"url":             f.URL,
				"name":            f.Name,
				"status":          f.Status,
				"size":            f.Size,
				"error":           f.Error,
				"itemId":          f.ItemID,
				"media":           f.Media,
				"ref_user_msg_id": userMsgID,
			})
		}
	}

	var upstreamMessages []map[string]interface{}
	hasPromptTools := len(tools) > 0

	// 提取 system 消息并转为 user+assistant 对注入对话开头
	// z.ai 会忽略 system 角色消息
	var systemTexts []string
	var nonSystemMessages []model.Message
	for _, msg := range messages {
		if msg.Role == "system" {
			text, _ := msg.ParseContent()
			if text != "" {
				systemTexts = append(systemTexts, text)
			}
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	for _, msg := range nonSystemMessages {
		if hasPromptTools {
			// prompt 注入模式：将 tool_calls / tool 结果转为纯文本
			if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				text, _ := msg.ParseContent()
				callText := builtintools.ConvertToolCallToText(msg.ToolCalls)
				if text != "" {
					text = text + "\n" + callText
				} else {
					text = callText
				}
				upstreamMessages = append(upstreamMessages, map[string]interface{}{
					"role":    "assistant",
					"content": text,
				})
				continue
			}
			if msg.Role == "tool" {
				text, _ := msg.ParseContent()
				upstreamMessages = append(upstreamMessages, map[string]interface{}{
					"role":    "user",
					"content": builtintools.ConvertToolResultToText(msg.ToolCallID, text),
				})
				continue
			}
		}
		upstreamMessages = append(upstreamMessages, msg.ToUpstreamMessage(urlToFileID))
	}

	// 工具注入：通过 user+assistant 对话注入工具定义
	// z.ai 会忽略 system 角色消息，因此使用 user/assistant 模拟注入
	if len(tools) > 0 {
		toolSystemPrompt := builtintools.BuildToolSystemPrompt(tools, toolChoice)
		if toolSystemPrompt != "" {
			logger.LogDebug("[ToolPrompt] Injecting tool system prompt (%d bytes, %d tools)", len(toolSystemPrompt), len(tools))
			userPromptMsg := map[string]interface{}{
				"role":    "user",
				"content": toolSystemPrompt,
			}
			assistantAckMsg := map[string]interface{}{
				"role":    "assistant",
				"content": "好的，我已了解可用工具。当需要使用工具时，我会直接输出 <tool_call> 标签进行调用。",
			}
			upstreamMessages = append([]map[string]interface{}{userPromptMsg, assistantAckMsg}, upstreamMessages...)
		}
	}

	// system 消息注入：通过 user+assistant 对注入对话开头
	if len(systemTexts) > 0 {
		combinedSystem := strings.Join(systemTexts, "\n\n")
		logger.LogDebug("[System] Injecting system message as user+assistant pair (%d bytes)", len(combinedSystem))
		systemUserMsg := map[string]interface{}{
			"role":    "user",
			"content": "[System Instructions]\n" + combinedSystem,
		}
		systemAssistantMsg := map[string]interface{}{
			"role":    "assistant",
			"content": "Understood. I will follow these instructions.",
		}
		upstreamMessages = append([]map[string]interface{}{systemUserMsg, systemAssistantMsg}, upstreamMessages...)
	}

	body := map[string]interface{}{
		"stream":           true,
		"model":            targetModel,
		"messages":         upstreamMessages,
		"signature_prompt": latestUserContent,
		"params":           map[string]interface{}{},
		"features": map[string]interface{}{
			"image_generation": false,
			"web_search":       false,
			"auto_web_search":  autoWebSearch,
			"preview_mode":     true,
			"enable_thinking":  enableThinking,
		},
		"chat_id": chatID,
		"id":      uuid.New().String(),
	}

	// Pass through optional request parameters
	if reqParams.Temperature != nil {
		body["temperature"] = *reqParams.Temperature
	}
	if reqParams.TopP != nil {
		body["top_p"] = *reqParams.TopP
	}
	if reqParams.MaxTokens != nil {
		body["max_tokens"] = *reqParams.MaxTokens
	}
	if reqParams.FrequencyPenalty != nil {
		body["frequency_penalty"] = *reqParams.FrequencyPenalty
	}
	if reqParams.PresencePenalty != nil {
		body["presence_penalty"] = *reqParams.PresencePenalty
	}
	if reqParams.Seed != nil {
		body["seed"] = *reqParams.Seed
	}
	if reqParams.ToolStream {
		body["tool_stream"] = true
	}

	if len(mcpServers) > 0 {
		body["mcp_servers"] = mcpServers
	}

	if len(filesData) > 0 {
		body["files"] = filesData
		body["current_user_message_id"] = userMsgID
	}

	bodyBytes, _ := json.Marshal(body)

	// Debug: log the messages being sent
	if len(tools) > 0 {
		for i, msg := range upstreamMessages {
			role, _ := msg["role"].(string)
			content, _ := msg["content"].(string)
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			logger.LogDebug("[ToolPrompt] msg[%d] role=%s content=%s", i, role, content)
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-FE-Version", version.GetFeVersion())
	req.Header.Set("X-Signature", signature)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Origin", "https://chat.z.ai")
	req.Header.Set("Referer", fmt.Sprintf("https://chat.z.ai/c/%s", uuid.New().String()))
	req.Header.Set("User-Agent", uarand.GetRandom())

	client := &http.Client{
		Timeout: 300 * time.Second, // 5 min overall timeout for long streaming responses
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}

	return resp, targetModel, nil
}
