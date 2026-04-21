package handler

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"zai-proxy/internal/auth"
	"zai-proxy/internal/filter"
	"zai-proxy/internal/logger"
	"zai-proxy/internal/model"
	toolset "zai-proxy/internal/tools"
)

func HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("x-api-key")
	if token == "" {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if token == "free" {
		anonymousToken, err := auth.GetAnonymousToken()
		if err != nil {
			logger.LogError("Failed to get anonymous token: %v", err)
			http.Error(w, "Failed to get anonymous token", http.StatusInternalServerError)
			return
		}
		token = anonymousToken
	}

	var req model.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		req.Model = "GLM-4.6"
	}

	effectiveTools := toolset.ResolveEffectiveTools(req.Model, req.Tools)
	completionID := fmt.Sprintf("chatcmpl-%s", uuid.New().String()[:29])
	reqParams := req.ToRequestParams()

	if !req.Stream {
		turn, modelName, err := runAutoToolLoop(token, req.Messages, req.Model, effectiveTools.Tools, req.ToolChoice, effectiveTools.InjectedBuiltinNames, "call_", reqParams)
		if err != nil {
			handleChatUpstreamError(w, err)
			return
		}
		writeOpenAINonStreamTurn(w, completionID, modelName, turn)
		return
	}

	if effectiveTools.HasInjectedBuiltins() {
		turn, modelName, err := runAutoToolLoop(token, req.Messages, req.Model, effectiveTools.Tools, req.ToolChoice, effectiveTools.InjectedBuiltinNames, "call_", reqParams)
		if err != nil {
			handleChatUpstreamError(w, err)
			return
		}
		writeOpenAIStreamTurn(w, completionID, modelName, turn)
		return
	}

	resp, modelName, err := makeUpstreamRequest(token, req.Messages, req.Model, effectiveTools.Tools, req.ToolChoice, reqParams)
	if err != nil {
		logger.LogError("Upstream request failed: %v", err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
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
		http.Error(w, "Upstream error", resp.StatusCode)
		return
	}

	handleStreamResponse(w, resp.Body, completionID, modelName, effectiveTools.Tools)
}

func handleChatUpstreamError(w http.ResponseWriter, err error) {
	var statusErr *upstreamStatusError
	if errors.As(err, &statusErr) {
		logger.LogError("Upstream error: status=%d, body=%s", statusErr.StatusCode, statusErr.Body)
		http.Error(w, "Upstream error", statusErr.StatusCode)
		return
	}

	logger.LogError("Upstream request failed: %v", err)
	http.Error(w, "Upstream error", http.StatusBadGateway)
}

func handleStreamResponse(w http.ResponseWriter, body io.ReadCloser, completionID, modelName string, tools []model.Tool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	hasContent := false
	searchRefFilter := filter.NewSearchRefFilter()
	thinkingFilter := &filter.ThinkingFilter{}
	pendingSourcesMarkdown := ""
	pendingImageSearchMarkdown := ""
	totalContentOutputLength := 0
	hasToolCalls := false
	var collectedToolCalls []model.ToolCall
	promptToolBuffer := "" // 用于 prompt 注入模式下缓冲 answer 文本以检测 <tool_call>

	for scanner.Scan() {
		line := scanner.Text()
		logger.LogDebug("[Upstream] %s", line)

		if !strings.HasPrefix(line, "data: ") {
			logger.LogDebug("[DEBUG-Stream] non-data line: %s", truncate(line, 200))
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var upstreamData model.UpstreamData
		if err := json.Unmarshal([]byte(payload), &upstreamData); err != nil {
			logger.LogDebug("[DEBUG-Stream] JSON parse error: %v, payload=%s", err, truncate(payload, 300))
			continue
		}

		logger.LogDebug("[DEBUG-Stream] phase=%s delta_content_len=%d edit_content_len=%d", upstreamData.Data.Phase, len(upstreamData.Data.DeltaContent), len(upstreamData.Data.EditContent))

		if upstreamData.Data.Phase == "done" {
			break
		}

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
					hasContent = true
					chunk := model.ChatCompletionChunk{
						ID:      completionID,
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   modelName,
						Choices: []model.Choice{{
							Index:        0,
							Delta:        &model.Delta{ReasoningContent: reasoningContent},
							FinishReason: nil,
						}},
					}
					data := marshalChunk(chunk)
					if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
					flusher.Flush()
				}
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
				pendingSourcesMarkdown = searchRefFilter.GetSearchResultsMarkdown()
			}
			continue
		}
		if editContent != "" && strings.Contains(editContent, `"search_image"`) {
			textBeforeBlock := filter.ExtractTextBeforeGlmBlock(editContent)
			if textBeforeBlock != "" {
				textBeforeBlock = searchRefFilter.Process(textBeforeBlock)
				if textBeforeBlock != "" {
					hasContent = true
					chunk := model.ChatCompletionChunk{
						ID:      completionID,
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   modelName,
						Choices: []model.Choice{{
							Index:        0,
							Delta:        &model.Delta{Content: textBeforeBlock},
							FinishReason: nil,
						}},
					}
					data := marshalChunk(chunk)
					if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
					flusher.Flush()
				}
			}
			if results := filter.ParseImageSearchResults(editContent); len(results) > 0 {
				pendingImageSearchMarkdown = filter.FormatImageSearchResults(results)
			}
			continue
		}
		if editContent != "" && strings.Contains(editContent, `"mcp"`) {
			textBeforeBlock := filter.ExtractTextBeforeGlmBlock(editContent)
			if textBeforeBlock != "" {
				textBeforeBlock = searchRefFilter.Process(textBeforeBlock)
				if textBeforeBlock != "" {
					hasContent = true
					chunk := model.ChatCompletionChunk{
						ID:      completionID,
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   modelName,
						Choices: []model.Choice{{
							Index:        0,
							Delta:        &model.Delta{Content: textBeforeBlock},
							FinishReason: nil,
						}},
					}
					data := marshalChunk(chunk)
					if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
					flusher.Flush()
				}
			}
			continue
		}
		if editContent != "" && filter.IsSearchToolCall(editContent, upstreamData.Data.Phase) {
			continue
		}
		// 检测用户定义的函数调用（tool_call 阶段，非 mcp/search）
		if upstreamData.Data.Phase == "tool_call" && editContent != "" {
			logger.LogInfo("[ToolCall] phase=%s edit_content=%s", upstreamData.Data.Phase, editContent)
		}
		if len(tools) > 0 && editContent != "" && filter.IsFunctionToolCall(editContent, upstreamData.Data.Phase) {
			if toolCalls := filter.ParseFunctionToolCalls(editContent); len(toolCalls) > 0 {
				for i := range toolCalls {
					if toolCalls[i].ID == "" {
						toolCalls[i].ID = fmt.Sprintf("call_%s", uuid.New().String()[:24])
					}
					toolCalls[i].Index = i
				}
				collectedToolCalls = toolCalls
				hasToolCalls = true

				for _, tc := range toolCalls {
					hasContent = true
					chunk := model.ChatCompletionChunk{
						ID:      completionID,
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   modelName,
						Choices: []model.Choice{{
							Index: 0,
							Delta: &model.Delta{
								ToolCalls: []model.ToolCall{tc},
							},
							FinishReason: nil,
						}},
					}
					data := marshalChunk(chunk)
					if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
					flusher.Flush()
				}
			}
			continue
		}

		if pendingSourcesMarkdown != "" {
			hasContent = true
			chunk := model.ChatCompletionChunk{
				ID:      completionID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   modelName,
				Choices: []model.Choice{{
					Index:        0,
					Delta:        &model.Delta{Content: pendingSourcesMarkdown},
					FinishReason: nil,
				}},
			}
			data := marshalChunk(chunk)
			if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
			flusher.Flush()
			pendingSourcesMarkdown = ""
		}
		if pendingImageSearchMarkdown != "" {
			hasContent = true
			chunk := model.ChatCompletionChunk{
				ID:      completionID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   modelName,
				Choices: []model.Choice{{
					Index:        0,
					Delta:        &model.Delta{Content: pendingImageSearchMarkdown},
					FinishReason: nil,
				}},
			}
			data := marshalChunk(chunk)
			if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
			flusher.Flush()
			pendingImageSearchMarkdown = ""
		}

		content := ""
		reasoningContent := ""

		if thinkingRemaining := thinkingFilter.Flush(); thinkingRemaining != "" {
			thinkingFilter.LastOutputChunk = thinkingRemaining
			processedRemaining := searchRefFilter.Process(thinkingRemaining)
			if processedRemaining != "" {
				hasContent = true
				chunk := model.ChatCompletionChunk{
					ID:      completionID,
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   modelName,
					Choices: []model.Choice{{
						Index:        0,
						Delta:        &model.Delta{ReasoningContent: processedRemaining},
						FinishReason: nil,
					}},
				}
				data := marshalChunk(chunk)
				if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
				flusher.Flush()
			}
		}

		if pendingSourcesMarkdown != "" && thinkingFilter.HasSeenFirstThinking {
			hasContent = true
			chunk := model.ChatCompletionChunk{
				ID:      completionID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   modelName,
				Choices: []model.Choice{{
					Index:        0,
					Delta:        &model.Delta{ReasoningContent: pendingSourcesMarkdown},
					FinishReason: nil,
				}},
			}
			data := marshalChunk(chunk)
			if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
			flusher.Flush()
			pendingSourcesMarkdown = ""
		}

		if upstreamData.Data.Phase == "answer" && upstreamData.Data.DeltaContent != "" {
			content = upstreamData.Data.DeltaContent
		} else if upstreamData.Data.Phase == "answer" && editContent != "" {
			if strings.Contains(editContent, "</details>") {
				reasoningContent = thinkingFilter.ExtractIncrementalThinking(editContent)

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
			fullContent := editContent
			fullContentRunes := []rune(fullContent)

			if len(fullContentRunes) > totalContentOutputLength {
				content = string(fullContentRunes[totalContentOutputLength:])
				totalContentOutputLength = len(fullContentRunes)
			} else {
				content = fullContent
			}
		}

		if reasoningContent != "" {
			reasoningContent = searchRefFilter.Process(reasoningContent) + searchRefFilter.Flush()
		}
		if reasoningContent != "" {
			hasContent = true
			chunk := model.ChatCompletionChunk{
				ID:      completionID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   modelName,
				Choices: []model.Choice{{
					Index:        0,
					Delta:        &model.Delta{ReasoningContent: reasoningContent},
					FinishReason: nil,
				}},
			}
			data := marshalChunk(chunk)
			if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
			flusher.Flush()
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

		// prompt 注入模式：缓冲 answer 文本，检测 <tool_call> 块
		if len(tools) > 0 {
			promptToolBuffer += content
			// 循环提取完整的 <tool_call>...</tool_call> 块
			for {
				openIdx := strings.Index(promptToolBuffer, "<tool_call>")
				if openIdx == -1 {
					// 无 <tool_call> 标签，全部安全输出
					break
				}
				// 输出 <tool_call> 之前的安全文本
				if openIdx > 0 {
					safeContent := promptToolBuffer[:openIdx]
					promptToolBuffer = promptToolBuffer[openIdx:]
					if safeContent != "" {
						sendContentChunk(w, flusher, completionID, modelName, safeContent)
					}
				}
				// 查找闭合标签：</tool_call>、</think>、或下一个 <tool_call>
				afterOpen := promptToolBuffer[len("<tool_call>"):]
				closeIdx := strings.Index(promptToolBuffer, "</tool_call>")
				thinkCloseIdx := strings.Index(afterOpen, "</think>")
				nextOpenIdx := strings.Index(afterOpen, "<tool_call>")

				// 选择最近的闭合位置
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
					// 下一个 <tool_call> 隐式关闭当前块
					candidate := len("<tool_call>") + nextOpenIdx
					if blockEnd == -1 || candidate < blockEnd {
						blockEnd = candidate
					}
				}

				if blockEnd == -1 {
					// 未找到任何闭合标记，等待更多数据
					break
				}

				// 提取完整块
				block := promptToolBuffer[:blockEnd]
				promptToolBuffer = promptToolBuffer[blockEnd:]

				// 解析 tool call
				_, toolCalls := filter.ExtractPromptToolCalls(block)
				if len(toolCalls) > 0 {
					collectedToolCalls = append(collectedToolCalls, toolCalls...)
					hasToolCalls = true
					for _, tc := range toolCalls {
						chunk := model.ChatCompletionChunk{
							ID:      completionID,
							Object:  "chat.completion.chunk",
							Created: time.Now().Unix(),
							Model:   modelName,
							Choices: []model.Choice{{
								Index: 0,
								Delta: &model.Delta{
									ToolCalls: []model.ToolCall{tc},
								},
								FinishReason: nil,
							}},
						}
						data := marshalChunk(chunk)
						if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
						flusher.Flush()
					}
				}
			}
			continue
		}

		chunk := model.ChatCompletionChunk{
			ID:      completionID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   modelName,
			Choices: []model.Choice{{
				Index:        0,
				Delta:        &model.Delta{Content: content},
				FinishReason: nil,
			}},
		}

		data := marshalChunk(chunk)
		if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		logger.LogError("[Upstream] scanner error: %v", err)
	}

	// prompt 注入模式：flush 缓冲区中剩余的文本
	if promptToolBuffer != "" {
		// 尝试最后一次提取 tool calls
		cleanContent, toolCalls := filter.ExtractPromptToolCalls(promptToolBuffer)
		if len(toolCalls) > 0 {
			collectedToolCalls = append(collectedToolCalls, toolCalls...)
			hasToolCalls = true
			for _, tc := range toolCalls {
				chunk := model.ChatCompletionChunk{
					ID:      completionID,
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   modelName,
					Choices: []model.Choice{{
						Index: 0,
						Delta: &model.Delta{
							ToolCalls: []model.ToolCall{tc},
						},
						FinishReason: nil,
					}},
				}
				data := marshalChunk(chunk)
				if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
				flusher.Flush()
			}
		}
		if cleanContent != "" {
			sendContentChunk(w, flusher, completionID, modelName, cleanContent)
			hasContent = true
		}
		promptToolBuffer = ""
	}

	if remaining := searchRefFilter.Flush(); remaining != "" {
		hasContent = true
		chunk := model.ChatCompletionChunk{
			ID:      completionID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   modelName,
			Choices: []model.Choice{{
				Index:        0,
				Delta:        &model.Delta{Content: remaining},
				FinishReason: nil,
			}},
		}
		data := marshalChunk(chunk)
		if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
		flusher.Flush()
	}

	if !hasContent {
		logger.LogError("Stream response 200 but no content received")
	}

	stopReason := "stop"
	if hasToolCalls && len(collectedToolCalls) > 0 {
		stopReason = "tool_calls"
	}
	finalChunk := model.ChatCompletionChunk{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.Choice{{
			Index:        0,
			Delta:        &model.Delta{},
			FinishReason: &stopReason,
		}},
		Usage: &model.Usage{},
	}

	data, _ := json.Marshal(finalChunk)
	if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleNonStreamResponse(w http.ResponseWriter, body io.ReadCloser, completionID, modelName string, tools []model.Tool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var chunks []string
	var reasoningChunks []string
	thinkingFilter := &filter.ThinkingFilter{}
	searchRefFilter := filter.NewSearchRefFilter()
	hasThinking := false
	pendingSourcesMarkdown := ""
	pendingImageSearchMarkdown := ""
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
			logger.LogDebug("[DEBUG-NonStream] JSON parse error: %v, payload=%s", err, truncate(payload, 200))
			continue
		}

		logger.LogDebug("[DEBUG-NonStream] phase=%s delta_content_len=%d edit_content_len=%d", upstreamData.Data.Phase, len(upstreamData.Data.DeltaContent), len(upstreamData.Data.EditContent))

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
				pendingSourcesMarkdown = searchRefFilter.GetSearchResultsMarkdown()
			}
			continue
		}
		if editContent != "" && strings.Contains(editContent, `"search_image"`) {
			textBeforeBlock := filter.ExtractTextBeforeGlmBlock(editContent)
			if textBeforeBlock != "" {
				chunks = append(chunks, textBeforeBlock)
			}
			if results := filter.ParseImageSearchResults(editContent); len(results) > 0 {
				pendingImageSearchMarkdown = filter.FormatImageSearchResults(results)
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
		// 检测用户定义的函数调用
		if upstreamData.Data.Phase == "tool_call" && editContent != "" {
			logger.LogInfo("[ToolCall] phase=%s edit_content=%s", upstreamData.Data.Phase, editContent)
		}
		if len(tools) > 0 && editContent != "" && filter.IsFunctionToolCall(editContent, upstreamData.Data.Phase) {
			if toolCalls := filter.ParseFunctionToolCalls(editContent); len(toolCalls) > 0 {
				for i := range toolCalls {
					if toolCalls[i].ID == "" {
						toolCalls[i].ID = fmt.Sprintf("call_%s", uuid.New().String()[:24])
					}
					toolCalls[i].Index = i
				}
				collectedToolCalls = toolCalls
			}
			continue
		}

		if pendingSourcesMarkdown != "" {
			if hasThinking {
				reasoningChunks = append(reasoningChunks, pendingSourcesMarkdown)
			} else {
				chunks = append(chunks, pendingSourcesMarkdown)
			}
			pendingSourcesMarkdown = ""
		}
		if pendingImageSearchMarkdown != "" {
			chunks = append(chunks, pendingImageSearchMarkdown)
			pendingImageSearchMarkdown = ""
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

	// prompt 注入模式：从 answer 文本中提取 <tool_call> 块
	if len(tools) > 0 && len(collectedToolCalls) == 0 {
		cleanContent, promptToolCalls := filter.ExtractPromptToolCalls(fullContent)
		if len(promptToolCalls) > 0 {
			collectedToolCalls = promptToolCalls
			fullContent = cleanContent
		}
	}

	if fullContent == "" && len(collectedToolCalls) == 0 {
		logger.LogError("Non-stream response 200 but no content received")
	}

	stopReason := "stop"
	if len(collectedToolCalls) > 0 {
		stopReason = "tool_calls"
	}
	response := model.ChatCompletionResponse{
		ID:      completionID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.Choice{{
			Index: 0,
			Message: &model.MessageResp{
				Role:             "assistant",
				Content:          fullContent,
				ReasoningContent: fullReasoning,
				ToolCalls:        collectedToolCalls,
			},
			FinishReason: &stopReason,
		}},
		Usage: model.Usage{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendContentChunk 发送一个 content SSE chunk
func sendContentChunk(w http.ResponseWriter, flusher http.Flusher, completionID, modelName, content string) {
	chunk := model.ChatCompletionChunk{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.Choice{{
			Index:        0,
			Delta:        &model.Delta{Content: content},
			FinishReason: nil,
		}},
	}
	data := marshalChunk(chunk)
	if data != nil { fmt.Fprintf(w, "data: %s\n\n", data) }
	flusher.Flush()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// marshalChunk safely marshals a chunk to JSON, logging errors instead of silently ignoring them
func marshalChunk(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		logger.LogError("Failed to marshal stream chunk: %v", err)
		return nil
	}
	return data
}
