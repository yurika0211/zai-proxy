package model

import "encoding/json"

// OpenAI 格式的消息内容项
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

// Tool 工具定义（OpenAI 兼容）
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction 函数定义
type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// ToolCall 模型返回的工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
	Index    int          `json:"index"`
}

// FunctionCall 函数调用（名称 + 参数 JSON 字符串）
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message 支持纯文本和多模态内容
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`              // string 或 []ContentPart
	ToolCallID string      `json:"tool_call_id,omitempty"` // role: "tool" 时使用
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`   // role: "assistant" 时使用
}

// 解析消息内容，返回文本和图片URL列表
func (m *Message) ParseContent() (text string, imageURLs []string) {
	switch content := m.Content.(type) {
	case string:
		return content, nil
	case []interface{}:
		for _, item := range content {
			if part, ok := item.(map[string]interface{}); ok {
				partType, _ := part["type"].(string)
				if partType == "text" {
					if t, ok := part["text"].(string); ok {
						text += t
					}
				} else if partType == "image_url" {
					if imgURL, ok := part["image_url"].(map[string]interface{}); ok {
						if url, ok := imgURL["url"].(string); ok {
							imageURLs = append(imageURLs, url)
						}
					}
				}
			}
		}
	}
	return text, imageURLs
}

// 转换为上游消息格式，支持多模态
func (m *Message) ToUpstreamMessage(urlToFileID map[string]string) map[string]interface{} {
	// tool 消息：包含 tool_call_id
	if m.Role == "tool" {
		msg := map[string]interface{}{
			"role":         m.Role,
			"content":      m.Content,
			"tool_call_id": m.ToolCallID,
		}
		return msg
	}

	// assistant 消息带 tool_calls
	if m.Role == "assistant" && len(m.ToolCalls) > 0 {
		msg := map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		}
		var toolCalls []map[string]interface{}
		for _, tc := range m.ToolCalls {
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}
		msg["tool_calls"] = toolCalls
		return msg
	}

	text, imageURLs := m.ParseContent()

	// 无图片，返回纯文本
	if len(imageURLs) == 0 {
		return map[string]interface{}{
			"role":    m.Role,
			"content": text,
		}
	}

	// 有图片，构建多模态内容
	var content []interface{}
	if text != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": text,
		})
	}
	for _, imgURL := range imageURLs {
		if fileID, ok := urlToFileID[imgURL]; ok {
			content = append(content, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": fileID,
				},
			})
		}
	}

	return map[string]interface{}{
		"role":    m.Role,
		"content": content,
	}
}

type ChatRequest struct {
	Model      string      `json:"model"`
	Messages   []Message   `json:"messages"`
	Stream     bool        `json:"stream"`
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"`
}

type ChatCompletionChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int          `json:"index"`
	Delta        *Delta       `json:"delta,omitempty"`
	Message      *MessageResp `json:"message,omitempty"`
	FinishReason *string      `json:"finish_reason"`
}

type Delta struct {
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

type MessageResp struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Index int    `json:"index"`
	RefID string `json:"ref_id"`
}

// ImageSearchResult 图片搜索结果
type ImageSearchResult struct {
	Title     string `json:"title"`
	Link      string `json:"link"`
	Thumbnail string `json:"thumbnail"`
}

// UpstreamData 上游返回的数据结构
type UpstreamData struct {
	Type string `json:"type"`
	Data struct {
		DeltaContent string `json:"delta_content"`
		EditContent  string `json:"edit_content"`
		Phase        string `json:"phase"`
		Done         bool   `json:"done"`
	} `json:"data"`
}

func (u *UpstreamData) GetEditContent() string {
	editContent := u.Data.EditContent
	if editContent == "" {
		return ""
	}

	if len(editContent) > 0 && editContent[0] == '"' {
		var unescaped string
		if err := json.Unmarshal([]byte(editContent), &unescaped); err == nil {
			return unescaped
		}
	}

	return editContent
}
