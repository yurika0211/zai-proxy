package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"zai-proxy/internal/filter"
	"zai-proxy/internal/logger"
	"zai-proxy/internal/model"
	toolruntime "zai-proxy/internal/tools"
	"zai-proxy/internal/upstream"
)

type assistantTurn struct {
	Content          string
	ReasoningContent string
	ToolCalls        []model.ToolCall
}

type upstreamStatusError struct {
	StatusCode int
	Body       string
}

func (e *upstreamStatusError) Error() string {
	return fmt.Sprintf("upstream returned status %d", e.StatusCode)
}

var makeUpstreamRequest = upstream.MakeUpstreamRequest

const maxAutomaticBuiltinToolRounds = 4

func collectAssistantTurn(body io.ReadCloser, tools []model.Tool, idPrefix string) (assistantTurn, error) {
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
			logger.LogInfo("[DEBUG-Collect] JSON parse error: %v, payload=%s", err, truncate(payload, 200))
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
		if len(tools) > 0 && editContent != "" && filter.IsFunctionToolCall(editContent, upstreamData.Data.Phase) {
			if toolCalls := filter.ParseFunctionToolCalls(editContent); len(toolCalls) > 0 {
				assignToolCallMetadata(toolCalls, idPrefix)
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

	if len(tools) > 0 && len(collectedToolCalls) == 0 {
		cleanContent, promptToolCalls := filter.ExtractPromptToolCalls(fullContent)
		if len(promptToolCalls) > 0 {
			assignToolCallMetadata(promptToolCalls, idPrefix)
			collectedToolCalls = promptToolCalls
			fullContent = cleanContent
		}
	}

	if err := scanner.Err(); err != nil {
		return assistantTurn{}, err
	}

	return assistantTurn{
		Content:          fullContent,
		ReasoningContent: fullReasoning,
		ToolCalls:        collectedToolCalls,
	}, nil
}

func assignToolCallMetadata(toolCalls []model.ToolCall, idPrefix string) {
	for i := range toolCalls {
		if toolCalls[i].ID == "" {
			toolCalls[i].ID = fmt.Sprintf("%s%s", idPrefix, uuid.New().String()[:24])
		}
		toolCalls[i].Index = i
		if toolCalls[i].Type == "" {
			toolCalls[i].Type = "function"
		}
	}
}

func runAutoToolLoop(token string, messages []model.Message, requestModel string, effectiveTools []model.Tool, toolChoice interface{}, autoBuiltinNames map[string]struct{}, idPrefix string, reqParams model.RequestParams) (assistantTurn, string, error) {
	currentMessages := append([]model.Message(nil), messages...)
	lastModelName := ""

	for round := 0; round <= maxAutomaticBuiltinToolRounds; round++ {
		resp, modelName, err := makeUpstreamRequest(token, currentMessages, requestModel, effectiveTools, toolChoice, reqParams)
		if err != nil {
			return assistantTurn{}, modelName, err
		}
		lastModelName = modelName

		if resp.StatusCode != http.StatusOK {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			bodyStr := ""
			if readErr != nil {
				logger.LogError("Failed to read upstream error body: %v", readErr)
			} else {
				bodyStr = string(body)
			}
			if len(bodyStr) > 500 {
				bodyStr = bodyStr[:500]
			}
			return assistantTurn{}, modelName, &upstreamStatusError{StatusCode: resp.StatusCode, Body: bodyStr}
		}

		turn, err := collectAssistantTurn(resp.Body, effectiveTools, idPrefix)
		resp.Body.Close()
		if err != nil {
			return assistantTurn{}, modelName, err
		}

		if len(turn.ToolCalls) == 0 || len(autoBuiltinNames) == 0 {
			return turn, modelName, nil
		}

		if !allToolCallsAutoExecutable(turn.ToolCalls, autoBuiltinNames) {
			return turn, modelName, nil
		}

		if round == maxAutomaticBuiltinToolRounds {
			return assistantTurn{}, modelName, fmt.Errorf("automatic builtin tool execution exceeded %d rounds", maxAutomaticBuiltinToolRounds)
		}

		currentMessages = append(currentMessages, model.Message{
			Role:      "assistant",
			Content:   turn.Content,
			ToolCalls: turn.ToolCalls,
		})
		for _, tc := range turn.ToolCalls {
			currentMessages = append(currentMessages, model.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    toolruntime.ExecuteBuiltinTool(tc.Function.Name, tc.Function.Arguments),
			})
		}
	}

	return assistantTurn{}, lastModelName, fmt.Errorf("automatic builtin tool execution reached an unexpected terminal state")
}

func allToolCallsAutoExecutable(toolCalls []model.ToolCall, autoBuiltinNames map[string]struct{}) bool {
	if len(toolCalls) == 0 {
		return false
	}
	for _, tc := range toolCalls {
		if _, ok := autoBuiltinNames[tc.Function.Name]; !ok {
			return false
		}
	}
	return true
}

func writeOpenAINonStreamTurn(w http.ResponseWriter, completionID, modelName string, turn assistantTurn) {
	stopReason := "stop"
	if len(turn.ToolCalls) > 0 {
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
				Content:          turn.Content,
				ReasoningContent: turn.ReasoningContent,
				ToolCalls:        turn.ToolCalls,
			},
			FinishReason: &stopReason,
		}},
		Usage: model.Usage{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func writeOpenAIStreamTurn(w http.ResponseWriter, completionID, modelName string, turn assistantTurn) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	if turn.ReasoningContent != "" {
		chunk := model.ChatCompletionChunk{
			ID:      completionID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   modelName,
			Choices: []model.Choice{{
				Index:        0,
				Delta:        &model.Delta{ReasoningContent: turn.ReasoningContent},
				FinishReason: nil,
			}},
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	if turn.Content != "" {
		sendContentChunk(w, flusher, completionID, modelName, turn.Content)
	}

	for _, tc := range turn.ToolCalls {
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
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	stopReason := "stop"
	if len(turn.ToolCalls) > 0 {
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
	fmt.Fprintf(w, "data: %s\n\n", data)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeAnthropicNonStreamTurn(w http.ResponseWriter, messageID, requestModel string, turn assistantTurn) {
	var contentBlocks []model.AnthropicContentBlock

	if turn.ReasoningContent != "" {
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type:     "thinking",
			Thinking: turn.ReasoningContent,
		})
	}

	if turn.Content != "" {
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type: "text",
			Text: turn.Content,
		})
	}

	for _, tc := range turn.ToolCalls {
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: anthropicRawJSON(tc.Function.Arguments),
		})
	}

	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, model.AnthropicContentBlock{
			Type: "text",
			Text: "",
		})
	}

	stopReason := "end_turn"
	if len(turn.ToolCalls) > 0 {
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

func writeAnthropicStreamTurn(w http.ResponseWriter, messageID, requestModel string, turn assistantTurn) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAnthropicError(w, http.StatusInternalServerError, "api_error", "Streaming not supported")
		return
	}

	sendAnthropicSSE(w, flusher, "message_start", model.AnthropicMessageStart{
		Type: "message_start",
		Message: model.AnthropicResponse{
			ID:      messageID,
			Type:    "message",
			Role:    "assistant",
			Content: []model.AnthropicContentBlock{},
			Model:   requestModel,
			Usage:   model.AnthropicUsage{},
		},
	})

	contentBlockIndex := 0

	if turn.ReasoningContent != "" {
		sendAnthropicSSE(w, flusher, "content_block_start", model.AnthropicContentBlockStart{
			Type:         "content_block_start",
			Index:        contentBlockIndex,
			ContentBlock: model.AnthropicContentBlock{Type: "thinking", Thinking: ""},
		})
		sendAnthropicSSE(w, flusher, "content_block_delta", model.AnthropicContentBlockDelta{
			Type:  "content_block_delta",
			Index: contentBlockIndex,
			Delta: model.AnthropicContentBlockDelta2{Type: "thinking_delta", Thinking: turn.ReasoningContent},
		})
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type:  "content_block_stop",
			Index: contentBlockIndex,
		})
		contentBlockIndex++
	}

	if turn.Content != "" || (turn.ReasoningContent == "" && len(turn.ToolCalls) == 0) {
		sendAnthropicSSE(w, flusher, "content_block_start", model.AnthropicContentBlockStart{
			Type:         "content_block_start",
			Index:        contentBlockIndex,
			ContentBlock: model.AnthropicContentBlock{Type: "text", Text: ""},
		})
		if turn.Content != "" {
			sendAnthropicSSE(w, flusher, "content_block_delta", model.AnthropicContentBlockDelta{
				Type:  "content_block_delta",
				Index: contentBlockIndex,
				Delta: model.AnthropicContentBlockDelta2{Type: "text_delta", Text: turn.Content},
			})
		}
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type:  "content_block_stop",
			Index: contentBlockIndex,
		})
		contentBlockIndex++
	}

	for _, tc := range turn.ToolCalls {
		toolID := tc.ID
		if toolID == "" {
			toolID = fmt.Sprintf("toolu_%s", uuid.New().String()[:24])
		}
		sendAnthropicSSE(w, flusher, "content_block_start", model.AnthropicContentBlockStart{
			Type:  "content_block_start",
			Index: contentBlockIndex,
			ContentBlock: model.AnthropicContentBlock{
				Type:  "tool_use",
				ID:    toolID,
				Name:  tc.Function.Name,
				Input: json.RawMessage("{}"),
			},
		})
		sendAnthropicSSE(w, flusher, "content_block_delta", model.AnthropicContentBlockDelta{
			Type:  "content_block_delta",
			Index: contentBlockIndex,
			Delta: model.AnthropicContentBlockDelta2{Type: "input_json_delta", PartialJSON: tc.Function.Arguments},
		})
		sendAnthropicSSE(w, flusher, "content_block_stop", model.AnthropicContentBlockStop{
			Type:  "content_block_stop",
			Index: contentBlockIndex,
		})
		contentBlockIndex++
	}

	stopReason := "end_turn"
	if len(turn.ToolCalls) > 0 {
		stopReason = "tool_use"
	}

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
	sendAnthropicSSE(w, flusher, "message_stop", model.AnthropicMessageStop{Type: "message_stop"})
}

func anthropicRawJSON(arguments string) json.RawMessage {
	if strings.TrimSpace(arguments) == "" {
		return json.RawMessage("{}")
	}
	return json.RawMessage(arguments)
}
