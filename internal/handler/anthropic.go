package handler

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"zai-proxy/internal/auth"
	"zai-proxy/internal/filter"
	"zai-proxy/internal/logger"
	"zai-proxy/internal/model"
	toolset "zai-proxy/internal/tools"
)

// HandleMessages handles Anthropic Messages API requests (/v1/messages)
func HandleMessages(w http.ResponseWriter, r *http.Request) {
	// Extract token from x-api-key or Authorization header
	token := r.Header.Get("x-api-key")
	if token == "" {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if token == "" {
		writeAnthropicError(w, http.StatusUnauthorized, "authentication_error", "Missing API key")
		return
	}

	if token == "free" {
		anonymousToken, err := auth.GetAnonymousToken()
		if err != nil {
			logger.LogError("Failed to get anonymous token: %v", err)
			writeAnthropicError(w, http.StatusInternalServerError, "api_error", "Failed to get anonymous token")
			return
		}
		token = anonymousToken
	}

	var req model.AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}

	if req.MaxTokens == 0 {
		req.MaxTokens = 8192
	}

	thinkingEnabled := false
	if req.Thinking != nil && req.Thinking.Type == "enabled" {
		thinkingEnabled = true
	}

	messages, tools, toolChoice := convertAnthropicToInternal(req)
	resolvedModel, _ := model.ResolveClaudeModel(req.Model, thinkingEnabled)
	effectiveTools := toolset.ResolveEffectiveTools(resolvedModel, tools)
	messageID := fmt.Sprintf("msg_%s", uuid.New().String()[:24])
	reqParams := model.RequestParams{} // Anthropic pass-through not yet implemented

	if !req.Stream {
		turn, modelName, err := runAutoToolLoop(token, messages, resolvedModel, effectiveTools.Tools, toolChoice, effectiveTools.InjectedBuiltinNames, "toolu_", reqParams)
		if err != nil {
			handleAnthropicUpstreamError(w, err)
			return
		}
		writeAnthropicNonStreamTurn(w, messageID, req.Model, turn)
		_ = modelName
		return
	}

	if effectiveTools.HasInjectedBuiltins() {
		turn, _, err := runAutoToolLoop(token, messages, resolvedModel, effectiveTools.Tools, toolChoice, effectiveTools.InjectedBuiltinNames, "toolu_", reqParams)
		if err != nil {
			handleAnthropicUpstreamError(w, err)
			return
		}
		writeAnthropicStreamTurn(w, messageID, req.Model, turn)
		return
	}

	resp, modelName, err := makeUpstreamRequest(token, messages, resolvedModel, effectiveTools.Tools, toolChoice, reqParams)
	if err != nil {
		logger.LogError("Upstream request failed: %v", err)
		writeAnthropicError(w, http.StatusBadGateway, "api_error", "Upstream error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		bodyStr := ""
		if err != nil {
			logger.LogError("Failed to read upstream error body: %v", err)
		} else {
			bodyStr = string(body)
		}
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500]
		}
		logger.LogError("Upstream error: status=%d, body=%s", resp.StatusCode, bodyStr)
		writeAnthropicError(w, resp.StatusCode, "api_error", "Upstream error")
		return
	}

	handleAnthropicStream(w, resp.Body, messageID, modelName, req.Model, effectiveTools.Tools)
}

func handleAnthropicUpstreamError(w http.ResponseWriter, err error) {
	var statusErr *upstreamStatusError
	if errors.As(err, &statusErr) {
		logger.LogError("Upstream error: status=%d, body=%s", statusErr.StatusCode, statusErr.Body)
		writeAnthropicError(w, statusErr.StatusCode, "api_error", "Upstream error")
		return
	}

	logger.LogError("Upstream request failed: %v", err)
	writeAnthropicError(w, http.StatusBadGateway, "api_error", "Upstream error")
}

// convertAnthropicToInternal converts Anthropic request format to internal Message/Tool format
func convertAnthropicToInternal(req model.AnthropicRequest) ([]model.Message, []model.Tool, interface{}) {
	var messages []model.Message

	// Convert system field to a system role message
	if req.System != nil {
		systemText := ""
		switch s := req.System.(type) {
		case string:
			systemText = s
		case []interface{}:
			// Array of content blocks
			for _, item := range s {
				if block, ok := item.(map[string]interface{}); ok {
					if t, ok := block["text"].(string); ok {
						systemText += t
					}
				}
			}
		}
		if systemText != "" {
			messages = append(messages, model.Message{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	// Convert Anthropic messages to internal format
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			text, blocks := msg.ParseContent()
			if len(blocks) == 0 {
				// Simple text message
				messages = append(messages, model.Message{
					Role:    "user",
					Content: text,
				})
			} else {
				var userContent []interface{}
				flushUserContent := func() {
					if len(userContent) == 0 {
						return
					}
					messages = append(messages, model.Message{
						Role:    "user",
						Content: append([]interface{}(nil), userContent...),
					})
					userContent = nil
				}

				// Process content blocks - may contain text, image, or tool_result
				for _, block := range blocks {
					switch block.Type {
					case "text":
						userContent = append(userContent, map[string]interface{}{
							"type": "text",
							"text": block.Text,
						})
					case "image":
						if imageURL := anthropicImageBlockToDataURL(block); imageURL != "" {
							userContent = append(userContent, map[string]interface{}{
								"type":      "image_url",
								"image_url": map[string]interface{}{"url": imageURL},
							})
						}
					case "tool_result":
						flushUserContent()
						messages = append(messages, model.Message{
							Role:       "tool",
							Content:    extractAnthropicToolResultContent(block.Content),
							ToolCallID: block.ToolUseID,
						})
					}
				}

				flushUserContent()
			}

		case "assistant":
			_, blocks := msg.ParseContent()
			if len(blocks) == 0 {
				// Simple text
				text, _ := msg.ParseContent()
				messages = append(messages, model.Message{
					Role:    "assistant",
					Content: text,
				})
			} else {
				// Assistant message with content blocks
				var textContent string
				var toolCalls []model.ToolCall
				for _, block := range blocks {
					switch block.Type {
					case "text":
						textContent += block.Text
					case "thinking":
						// Skip thinking blocks in history - upstream doesn't need them
					case "tool_use":
						argsStr := "{}"
						if block.Input != nil {
							argsStr = string(block.Input)
						}
						toolCalls = append(toolCalls, model.ToolCall{
							ID:   block.ID,
							Type: "function",
							Function: model.FunctionCall{
								Name:      block.Name,
								Arguments: argsStr,
							},
						})
					}
				}
				messages = append(messages, model.Message{
					Role:      "assistant",
					Content:   textContent,
					ToolCalls: toolCalls,
				})
			}
		}
	}

	// Convert Anthropic tools to OpenAI format
	var tools []model.Tool
	for _, t := range req.Tools {
		name := anthropicToolName(t)
		description := anthropicToolDescription(t)
		parameters := anthropicToolInputSchema(t)
		tools = append(tools, model.Tool{
			Type: "function",
			Function: model.ToolFunction{
				Name:        name,
				Description: description,
				Parameters:  parameters,
			},
		})
	}

	// Convert tool_choice
	var toolChoice interface{}
	if req.ToolChoice != nil {
		switch tc := req.ToolChoice.(type) {
		case map[string]interface{}:
			tcType, _ := tc["type"].(string)
			switch tcType {
			case "auto":
				toolChoice = "auto"
			case "any":
				toolChoice = "required"
			case "none":
				toolChoice = "none"
			case "tool":
				if name, ok := tc["name"].(string); ok {
					toolChoice = map[string]interface{}{
						"type":     "function",
						"function": map[string]interface{}{"name": name},
					}
				}
			}
		}
	}

	return messages, tools, toolChoice
}

func anthropicImageBlockToDataURL(block model.AnthropicContentBlock) string {
	if block.Source == nil {
		return ""
	}
	if block.Source.Type != "base64" || block.Source.MediaType == "" || block.Source.Data == "" {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
}

func extractAnthropicToolResultContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var result strings.Builder
		for _, item := range c {
			if part, ok := item.(map[string]interface{}); ok {
				if t, ok := part["text"].(string); ok {
					result.WriteString(t)
				}
			}
		}
		if result.Len() > 0 {
			return result.String()
		}
		data, err := json.Marshal(c)
		if err == nil {
			return string(data)
		}
		return ""
	case map[string]interface{}:
		data, err := json.Marshal(c)
		if err == nil {
			return string(data)
		}
		return ""
	default:
		data, err := json.Marshal(c)
		if err == nil {
			return string(data)
		}
		return ""
	}
}

func anthropicToolName(t model.AnthropicTool) string {
	if t.Name != "" {
		return t.Name
	}
	switch {
	case strings.HasPrefix(t.Type, "bash_"):
		return "bash"
	case strings.HasPrefix(t.Type, "text_editor_"):
		return "str_replace_based_edit_tool"
	default:
		return ""
	}
}

func anthropicToolDescription(t model.AnthropicTool) string {
	if t.Description != "" {
		return t.Description
	}
	switch {
	case strings.HasPrefix(t.Type, "bash_"):
		return "Execute shell commands in a persistent bash session. Use input.command to run a command or input.restart to restart the session."
	case strings.HasPrefix(t.Type, "text_editor_"):
		return "View and edit text files. Supported commands include view, str_replace, create, insert, and undo_edit, all passed via input.command."
	default:
		return ""
	}
}

func anthropicToolInputSchema(t model.AnthropicTool) interface{} {
	if t.InputSchema != nil {
		return t.InputSchema
	}

	switch {
	case strings.HasPrefix(t.Type, "bash_"):
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The bash command to run in the persistent session.",
				},
				"restart": map[string]interface{}{
					"type":        "boolean",
					"description": "Set to true to restart the bash session instead of running a command.",
				},
			},
		}
	case strings.HasPrefix(t.Type, "text_editor_"):
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"view", "str_replace", "create", "insert", "undo_edit"},
					"description": "Text editor command to execute.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file or directory being read or edited.",
				},
				"view_range": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "integer"},
					"description": "Optional start and end line numbers for the view command.",
				},
				"old_str": map[string]interface{}{
					"type":        "string",
					"description": "Exact text to replace when using str_replace.",
				},
				"new_str": map[string]interface{}{
					"type":        "string",
					"description": "Replacement text for str_replace.",
				},
				"file_text": map[string]interface{}{
					"type":        "string",
					"description": "Content to write when using the create command.",
				},
				"insert_line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number after which to insert text for the insert command.",
				},
				"insert_text": map[string]interface{}{
					"type":        "string",
					"description": "Text to insert for the insert command.",
				},
			},
			"required": []string{"command", "path"},
		}
	default:
		return nil
	}
}

// handleAnthropicStream processes upstream SSE and converts to Anthropic streaming format
func handleAnthropicStream(w http.ResponseWriter, body io.ReadCloser, messageID, modelName, requestModel string, tools []model.Tool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAnthropicError(w, http.StatusInternalServerError, "api_error", "Streaming not supported")
		return
	}

	// Send message_start
	msgStart := model.AnthropicMessageStart{
		Type: "message_start",
		Message: model.AnthropicResponse{
			ID:         messageID,
			Type:       "message",
			Role:       "assistant",
			Content:    []model.AnthropicContentBlock{},
			Model:      requestModel,
			StopReason: "",
			Usage:      model.AnthropicUsage{InputTokens: 0, OutputTokens: 0},
		},
	}
	sendAnthropicSSE(w, flusher, "message_start", msgStart)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	searchRefFilter := filter.NewSearchRefFilter()
	thinkingFilter := &filter.ThinkingFilter{}

	contentBlockIndex := 0
	inThinkingBlock := false
	inTextBlock := false
	inToolUseBlock := false
	hasContent := false
	totalContentOutputLength := 0
	hasToolCalls := false
	var collectedToolCalls []model.ToolCall
	promptToolBuffer := ""

	for scanner.Scan() {
		line := scanner.Text()
		logger.LogDebug("[Anthropic-Upstream] %s", line)

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var upstreamData model.UpstreamData
		if err := json.Unmarshal([]byte(payload), &upstreamData); err != nil {
			continue
		}

		if upstreamData.Data.Phase == "done" {
			break
		}

		// Handle thinking phase
		if upstreamData.Data.Phase == "thinking" && upstreamData.Data.DeltaContent != "" {
			isNewThinkingRound := false
			if thinkingFilter.LastPhase != "" && thinkingFilter.LastPhase != "thinking" {
				thinkingFilter.ResetForNewRound()
				thinkingFilter.ThinkingRoundCount++
				isNewThinkingRound = true
			}
			thinkingFilter.LastPhase = "thinking"

			reasoningContent := thinkingFilter.ProcessThinking(upstreamData.Data.DeltaContent)

			if isNewThinkingRound && thinkingFilter.ThinkingRoundCount > 1 && reasoningContent != "" {
				reasoningContent = "\n\n" + reasoningContent
			}

			if reasoningContent != "" {
				thinkingFilter.LastOutputChunk = reasoningContent
				reasoningContent = searchRefFilter.Process(reasoningContent)

				if reasoningContent != "" {
					// Close previous non-thinking block if open
					if inTextBlock {
						sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
							Type: "content_block_stop", Index: contentBlockIndex,
						})
						contentBlockIndex++
						inTextBlock = false
					}

					// Start thinking block if not already in one
					if !inThinkingBlock {
						sendAnthropicSSE(w, flusher, "content_block_start", model.AnthropicContentBlockStart{
							Type:         "content_block_start",
							Index:        contentBlockIndex,
							ContentBlock: model.AnthropicContentBlock{Type: "thinking", Thinking: ""},
						})
						inThinkingBlock = true
					}

					hasContent = true
					sendAnthropicSSE(w, flusher, "content_block_delta", model.AnthropicContentBlockDelta{
						Type:  "content_block_delta",
						Index: contentBlockIndex,
						Delta: model.AnthropicContentBlockDelta2{Type: "thinking_delta", Thinking: reasoningContent},
					})
				}
			}
			continue
		}

		if upstreamData.Data.Phase != "" {
			thinkingFilter.LastPhase = upstreamData.Data.Phase
		}

		// Filter search results, image searches, mcp, etc.
		editContent := upstreamData.GetEditContent()
		if editContent != "" && filter.IsSearchResultContent(editContent) {
			if results := filter.ParseSearchResults(editContent); len(results) > 0 {
				searchRefFilter.AddSearchResults(results)
			}
			continue
		}
		if editContent != "" && strings.Contains(editContent, `"search_image"`) {
			textBeforeBlock := filter.ExtractTextBeforeGlmBlock(editContent)
			if textBeforeBlock != "" {
				emitAnthropicTextDelta(w, flusher, &contentBlockIndex, &inThinkingBlock, &inTextBlock, &inToolUseBlock, &hasContent, searchRefFilter.Process(textBeforeBlock))
			}
			continue
		}
		if editContent != "" && strings.Contains(editContent, `"mcp"`) {
			textBeforeBlock := filter.ExtractTextBeforeGlmBlock(editContent)
			if textBeforeBlock != "" {
				emitAnthropicTextDelta(w, flusher, &contentBlockIndex, &inThinkingBlock, &inTextBlock, &inToolUseBlock, &hasContent, searchRefFilter.Process(textBeforeBlock))
			}
			continue
		}
		if editContent != "" && filter.IsSearchToolCall(editContent, upstreamData.Data.Phase) {
			continue
		}

		// Handle function tool calls
		if len(tools) > 0 && editContent != "" && filter.IsFunctionToolCall(editContent, upstreamData.Data.Phase) {
			if toolCalls := filter.ParseFunctionToolCalls(editContent); len(toolCalls) > 0 {
				for i := range toolCalls {
					if toolCalls[i].ID == "" {
						toolCalls[i].ID = fmt.Sprintf("toolu_%s", uuid.New().String()[:24])
					}
				}
				collectedToolCalls = toolCalls
				hasToolCalls = true

				// Close thinking/text blocks
				if inThinkingBlock {
					sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
						Type: "content_block_stop", Index: contentBlockIndex,
					})
					contentBlockIndex++
					inThinkingBlock = false
				}
				if inTextBlock {
					sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
						Type: "content_block_stop", Index: contentBlockIndex,
					})
					contentBlockIndex++
					inTextBlock = false
				}

				for _, tc := range toolCalls {
					emitAnthropicToolUse(w, flusher, &contentBlockIndex, &inToolUseBlock, tc)
				}
			}
			continue
		}

		// Flush thinking filter
		if thinkingRemaining := thinkingFilter.Flush(); thinkingRemaining != "" {
			thinkingFilter.LastOutputChunk = thinkingRemaining
			processedRemaining := searchRefFilter.Process(thinkingRemaining)
			if processedRemaining != "" {
				if !inThinkingBlock {
					// Close text block if open
					if inTextBlock {
						sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
							Type: "content_block_stop", Index: contentBlockIndex,
						})
						contentBlockIndex++
						inTextBlock = false
					}
					sendAnthropicSSE(w, flusher, "content_block_start", model.AnthropicContentBlockStart{
						Type:         "content_block_start",
						Index:        contentBlockIndex,
						ContentBlock: model.AnthropicContentBlock{Type: "thinking", Thinking: ""},
					})
					inThinkingBlock = true
				}
				hasContent = true
				sendAnthropicSSE(w, flusher, "content_block_delta", model.AnthropicContentBlockDelta{
					Type:  "content_block_delta",
					Index: contentBlockIndex,
					Delta: model.AnthropicContentBlockDelta2{Type: "thinking_delta", Thinking: processedRemaining},
				})
			}
		}

		// Extract content
		content := ""
		if upstreamData.Data.Phase == "answer" && upstreamData.Data.DeltaContent != "" {
			content = upstreamData.Data.DeltaContent
		} else if upstreamData.Data.Phase == "answer" && editContent != "" {
			if strings.Contains(editContent, "</details>") {
				if idx := strings.Index(editContent, "</details>"); idx != -1 {
					afterDetails := editContent[idx+len("</details>"):]
					if strings.HasPrefix(afterDetails, "\n") {
						content = afterDetails[1:]
					} else {
						content = afterDetails
					}
					totalContentOutputLength = len([]rune(content))
				}
			}
		} else if (upstreamData.Data.Phase == "other" || upstreamData.Data.Phase == "tool_call") && editContent != "" {
			fullContentRunes := []rune(editContent)
			if len(fullContentRunes) > totalContentOutputLength {
				content = string(fullContentRunes[totalContentOutputLength:])
				totalContentOutputLength = len(fullContentRunes)
			} else {
				content = editContent
			}
		}

		if content == "" {
			continue
		}

		content = searchRefFilter.Process(content)
		if content == "" {
			continue
		}

		hasContent = true
		if upstreamData.Data.Phase == "answer" && upstreamData.Data.DeltaContent != "" {
			totalContentOutputLength += len([]rune(content))
		}

		// Prompt tool extraction: buffer answer text for <tool_call> detection
		if len(tools) > 0 {
			promptToolBuffer += content
			for {
				openIdx := strings.Index(promptToolBuffer, "<tool_call>")
				if openIdx == -1 {
					break
				}
				if openIdx > 0 {
					safeContent := promptToolBuffer[:openIdx]
					promptToolBuffer = promptToolBuffer[openIdx:]
					if safeContent != "" {
						emitAnthropicTextDelta(w, flusher, &contentBlockIndex, &inThinkingBlock, &inTextBlock, &inToolUseBlock, &hasContent, safeContent)
					}
				}
				afterOpen := promptToolBuffer[len("<tool_call>"):]
				closeIdx := strings.Index(promptToolBuffer, "</tool_call>")
				thinkCloseIdx := strings.Index(afterOpen, "</think>")
				nextOpenIdx := strings.Index(afterOpen, "<tool_call>")

				blockEnd := -1
				if closeIdx != -1 {
					blockEnd = closeIdx + len("</tool_call>")
				}
				if thinkCloseIdx != -1 {
					candidate := len("<tool_call>") + thinkCloseIdx + len("</think>")
					if blockEnd == -1 || candidate < blockEnd {
						blockEnd = candidate
					}
				}
				if nextOpenIdx != -1 {
					candidate := len("<tool_call>") + nextOpenIdx
					if blockEnd == -1 || candidate < blockEnd {
						blockEnd = candidate
					}
				}
				if blockEnd == -1 {
					break
				}

				block := promptToolBuffer[:blockEnd]
				promptToolBuffer = promptToolBuffer[blockEnd:]

				_, ptToolCalls := filter.ExtractPromptToolCalls(block)
				if len(ptToolCalls) > 0 {
					collectedToolCalls = append(collectedToolCalls, ptToolCalls...)
					hasToolCalls = true

					// Close thinking/text blocks before emitting tool use
					if inThinkingBlock {
						sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
							Type: "content_block_stop", Index: contentBlockIndex,
						})
						contentBlockIndex++
						inThinkingBlock = false
					}
					if inTextBlock {
						sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
							Type: "content_block_stop", Index: contentBlockIndex,
						})
						contentBlockIndex++
						inTextBlock = false
					}

					for _, tc := range ptToolCalls {
						tc.ID = fmt.Sprintf("toolu_%s", uuid.New().String()[:24])
						emitAnthropicToolUse(w, flusher, &contentBlockIndex, &inToolUseBlock, tc)
					}
				}
			}
			continue
		}

		emitAnthropicTextDelta(w, flusher, &contentBlockIndex, &inThinkingBlock, &inTextBlock, &inToolUseBlock, &hasContent, content)
	}

	if err := scanner.Err(); err != nil {
		logger.LogError("[Anthropic-Upstream] scanner error: %v", err)
	}

	// Flush remaining prompt tool buffer
	if promptToolBuffer != "" {
		cleanContent, ptToolCalls := filter.ExtractPromptToolCalls(promptToolBuffer)
		if len(ptToolCalls) > 0 {
			collectedToolCalls = append(collectedToolCalls, ptToolCalls...)
			hasToolCalls = true

			if inThinkingBlock {
				sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
					Type: "content_block_stop", Index: contentBlockIndex,
				})
				contentBlockIndex++
				inThinkingBlock = false
			}
			if inTextBlock {
				sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
					Type: "content_block_stop", Index: contentBlockIndex,
				})
				contentBlockIndex++
				inTextBlock = false
			}

			for _, tc := range ptToolCalls {
				tc.ID = fmt.Sprintf("toolu_%s", uuid.New().String()[:24])
				emitAnthropicToolUse(w, flusher, &contentBlockIndex, &inToolUseBlock, tc)
			}
		}
		if cleanContent != "" {
			emitAnthropicTextDelta(w, flusher, &contentBlockIndex, &inThinkingBlock, &inTextBlock, &inToolUseBlock, &hasContent, cleanContent)
		}
		promptToolBuffer = ""
	}

	// Flush search ref filter
	if remaining := searchRefFilter.Flush(); remaining != "" {
		emitAnthropicTextDelta(w, flusher, &contentBlockIndex, &inThinkingBlock, &inTextBlock, &inToolUseBlock, &hasContent, remaining)
	}

	if !hasContent && !hasToolCalls {
		logger.LogError("Anthropic stream response 200 but no content received")
	}

	// Close any open blocks
	if inThinkingBlock {
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type: "content_block_stop", Index: contentBlockIndex,
		})
		contentBlockIndex++
		inThinkingBlock = false
	}
	if inTextBlock {
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type: "content_block_stop", Index: contentBlockIndex,
		})
		contentBlockIndex++
		inTextBlock = false
	}
	if inToolUseBlock {
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type: "content_block_stop", Index: contentBlockIndex,
		})
		contentBlockIndex++
		inToolUseBlock = false
	}

	// Determine stop reason
	stopReason := "end_turn"
	if hasToolCalls {
		stopReason = "tool_use"
	}

	// Send message_delta with stop_reason and usage
	sendAnthropicSSE(w, flusher, "message_delta", model.AnthropicMessageDelta{
		Type: "message_delta",
		Delta: struct {
			StopReason   string  `json:"stop_reason"`
			StopSequence *string `json:"stop_sequence"`
		}{
			StopReason: stopReason,
		},
		Usage: model.AnthropicUsage{},
	})

	// Send message_stop
	sendAnthropicSSE(w, flusher, "message_stop", model.AnthropicMessageStop{Type: "message_stop"})

	// Suppress unused variable warnings
	_ = inThinkingBlock
	_ = inTextBlock
	_ = inToolUseBlock
	_ = contentBlockIndex
}

// handleAnthropicNonStream collects all upstream data and returns an Anthropic response
func handleAnthropicNonStream(w http.ResponseWriter, body io.ReadCloser, messageID, modelName, requestModel string, tools []model.Tool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var chunks []string
	var reasoningChunks []string
	thinkingFilter := &filter.ThinkingFilter{}
	searchRefFilter := filter.NewSearchRefFilter()
	hasThinking := false
	var collectedToolCalls []model.ToolCall

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var upstreamData model.UpstreamData
		if err := json.Unmarshal([]byte(payload), &upstreamData); err != nil {
			continue
		}

		if upstreamData.Data.Phase == "done" {
			break
		}

		if upstreamData.Data.Phase == "thinking" && upstreamData.Data.DeltaContent != "" {
			if thinkingFilter.LastPhase != "" && thinkingFilter.LastPhase != "thinking" {
				thinkingFilter.ResetForNewRound()
				thinkingFilter.ThinkingRoundCount++
				if thinkingFilter.ThinkingRoundCount > 1 {
					reasoningChunks = append(reasoningChunks, "\n\n")
				}
			}
			thinkingFilter.LastPhase = "thinking"
			hasThinking = true
			reasoningContent := thinkingFilter.ProcessThinking(upstreamData.Data.DeltaContent)
			if reasoningContent != "" {
				thinkingFilter.LastOutputChunk = reasoningContent
				reasoningChunks = append(reasoningChunks, reasoningContent)
			}
			continue
		}

		if upstreamData.Data.Phase != "" {
			thinkingFilter.LastPhase = upstreamData.Data.Phase
		}

		editContent := upstreamData.GetEditContent()
		if editContent != "" && filter.IsSearchResultContent(editContent) {
			if results := filter.ParseSearchResults(editContent); len(results) > 0 {
				searchRefFilter.AddSearchResults(results)
			}
			continue
		}
		if editContent != "" && strings.Contains(editContent, `"search_image"`) {
			textBeforeBlock := filter.ExtractTextBeforeGlmBlock(editContent)
			if textBeforeBlock != "" {
				chunks = append(chunks, textBeforeBlock)
			}
			continue
		}
		if editContent != "" && strings.Contains(editContent, `"mcp"`) {
			textBeforeBlock := filter.ExtractTextBeforeGlmBlock(editContent)
			if textBeforeBlock != "" {
				chunks = append(chunks, textBeforeBlock)
			}
			continue
		}
		if editContent != "" && filter.IsSearchToolCall(editContent, upstreamData.Data.Phase) {
			continue
		}
		if len(tools) > 0 && editContent != "" && filter.IsFunctionToolCall(editContent, upstreamData.Data.Phase) {
			if toolCalls := filter.ParseFunctionToolCalls(editContent); len(toolCalls) > 0 {
				for i := range toolCalls {
					if toolCalls[i].ID == "" {
						toolCalls[i].ID = fmt.Sprintf("toolu_%s", uuid.New().String()[:24])
					}
				}
				collectedToolCalls = toolCalls
			}
			continue
		}

		content := ""
		if upstreamData.Data.Phase == "answer" && upstreamData.Data.DeltaContent != "" {
			content = upstreamData.Data.DeltaContent
		} else if upstreamData.Data.Phase == "answer" && editContent != "" {
			if strings.Contains(editContent, "</details>") {
				reasoningContent := thinkingFilter.ExtractIncrementalThinking(editContent)
				if reasoningContent != "" {
					reasoningChunks = append(reasoningChunks, reasoningContent)
				}
				if idx := strings.Index(editContent, "</details>"); idx != -1 {
					afterDetails := editContent[idx+len("</details>"):]
					if strings.HasPrefix(afterDetails, "\n") {
						content = afterDetails[1:]
					} else {
						content = afterDetails
					}
				}
			}
		} else if (upstreamData.Data.Phase == "other" || upstreamData.Data.Phase == "tool_call") && editContent != "" {
			content = editContent
		}

		if content != "" {
			chunks = append(chunks, content)
		}
	}

	fullContent := strings.Join(chunks, "")
	fullContent = searchRefFilter.Process(fullContent) + searchRefFilter.Flush()
	fullReasoning := strings.Join(reasoningChunks, "")
	fullReasoning = searchRefFilter.Process(fullReasoning) + searchRefFilter.Flush()

	// Extract prompt tool calls from answer text
	if len(tools) > 0 && len(collectedToolCalls) == 0 {
		cleanContent, promptToolCalls := filter.ExtractPromptToolCalls(fullContent)
		if len(promptToolCalls) > 0 {
			collectedToolCalls = promptToolCalls
			fullContent = cleanContent
		}
	}

	// Build response content blocks
	var contentBlocks []model.AnthropicContentBlock

	if hasThinking && fullReasoning != "" {
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type:     "thinking",
			Thinking: fullReasoning,
		})
	}

	if fullContent != "" {
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type: "text",
			Text: fullContent,
		})
	}

	for _, tc := range collectedToolCalls {
		if tc.ID == "" {
			tc.ID = fmt.Sprintf("toolu_%s", uuid.New().String()[:24])
		}
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type: "text",
			Text: "",
		})
	}

	stopReason := "end_turn"
	if len(collectedToolCalls) > 0 {
		stopReason = "tool_use"
	}

	response := model.AnthropicResponse{
		ID:         messageID,
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		Model:      requestModel,
		StopReason: stopReason,
		Usage:      model.AnthropicUsage{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// emitAnthropicTextDelta sends a text content delta, managing block lifecycle
func emitAnthropicTextDelta(w http.ResponseWriter, flusher http.Flusher, contentBlockIndex *int, inThinkingBlock, inTextBlock, inToolUseBlock *bool, hasContent *bool, text string) {
	if text == "" {
		return
	}

	// Close thinking block if transitioning to text
	if *inThinkingBlock {
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type: "content_block_stop", Index: *contentBlockIndex,
		})
		*contentBlockIndex++
		*inThinkingBlock = false
	}

	// Close tool_use block if transitioning to text
	if *inToolUseBlock {
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type: "content_block_stop", Index: *contentBlockIndex,
		})
		*contentBlockIndex++
		*inToolUseBlock = false
	}

	// Start text block if not in one
	if !*inTextBlock {
		sendAnthropicSSE(w, flusher, "content_block_start", model.AnthropicContentBlockStart{
			Type:         "content_block_start",
			Index:        *contentBlockIndex,
			ContentBlock: model.AnthropicContentBlock{Type: "text", Text: ""},
		})
		*inTextBlock = true
	}

	*hasContent = true
	sendAnthropicSSE(w, flusher, "content_block_delta", model.AnthropicContentBlockDelta{
		Type:  "content_block_delta",
		Index: *contentBlockIndex,
		Delta: model.AnthropicContentBlockDelta2{Type: "text_delta", Text: text},
	})
}

// emitAnthropicToolUse sends a tool_use content block (start + input_json_delta + stop)
func emitAnthropicToolUse(w http.ResponseWriter, flusher http.Flusher, contentBlockIndex *int, inToolUseBlock *bool, tc model.ToolCall) {
	// Close previous tool_use block if open
	if *inToolUseBlock {
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type: "content_block_stop", Index: *contentBlockIndex,
		})
		*contentBlockIndex++
	}

	toolID := tc.ID
	if toolID == "" {
		toolID = fmt.Sprintf("toolu_%s", uuid.New().String()[:24])
	}

	// Send content_block_start with tool_use
	sendAnthropicSSE(w, flusher, "content_block_start", model.AnthropicContentBlockStart{
		Type:  "content_block_start",
		Index: *contentBlockIndex,
		ContentBlock: model.AnthropicContentBlock{
			Type:  "tool_use",
			ID:    toolID,
			Name:  tc.Function.Name,
			Input: json.RawMessage("{}"),
		},
	})
	*inToolUseBlock = true

	// Send input as a single delta
	sendAnthropicSSE(w, flusher, "content_block_delta", model.AnthropicContentBlockDelta{
		Type:  "content_block_delta",
		Index: *contentBlockIndex,
		Delta: model.AnthropicContentBlockDelta2{Type: "input_json_delta", PartialJSON: tc.Function.Arguments},
	})
}

// sendAnthropicSSE writes an SSE event in Anthropic format: "event: <type>\ndata: <json>\n\n"
func sendAnthropicSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.LogError("[Anthropic-SSE] marshal error: %v", err)
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	flusher.Flush()
}

// writeAnthropicError writes an error response in Anthropic format
func writeAnthropicError(w http.ResponseWriter, statusCode int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    errorType,
			"message": message,
		},
	})
}
