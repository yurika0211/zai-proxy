package model

import "encoding/json"

// AnthropicRequest represents a request to the Anthropic Messages API
type AnthropicRequest struct {
	Model      string             `json:"model"`
	MaxTokens  int                `json:"max_tokens"`
	System     interface{}        `json:"system,omitempty"` // string or []AnthropicContentBlock
	Messages   []AnthropicMessage `json:"messages"`
	Tools      []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice interface{}        `json:"tool_choice,omitempty"`
	Stream     bool               `json:"stream"`
	Thinking   *AnthropicThinking `json:"thinking,omitempty"`
}

// AnthropicThinking controls thinking/reasoning behavior
type AnthropicThinking struct {
	Type         string `json:"type"` // "enabled" or "disabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// AnthropicMessage represents a message in Anthropic format
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []AnthropicContentBlock
}

// ParseContent extracts text content from an Anthropic message.
// Content can be a plain string or an array of content blocks.
func (m *AnthropicMessage) ParseContent() (text string, blocks []AnthropicContentBlock) {
	switch c := m.Content.(type) {
	case string:
		return c, nil
	case []interface{}:
		for _, item := range c {
			raw, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var block AnthropicContentBlock
			if err := json.Unmarshal(raw, &block); err != nil {
				continue
			}
			blocks = append(blocks, block)
			if block.Type == "text" {
				text += block.Text
			}
		}
	}
	return text, blocks
}

// AnthropicContentBlock represents a content block in Anthropic messages
type AnthropicContentBlock struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// thinking block
	Thinking string `json:"thinking,omitempty"`

	// tool_use block
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result block
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"` // string or []AnthropicContentBlock
	IsError   bool        `json:"is_error,omitempty"`

	// image block
	Source *AnthropicImageSource `json:"source,omitempty"`
}

// AnthropicImageSource for base64 image content
type AnthropicImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/png" etc
	Data      string `json:"data"`
}

// AnthropicTool represents a tool definition in Anthropic format
type AnthropicTool struct {
	Type          string      `json:"type,omitempty"`
	Name          string      `json:"name"`
	Description   string      `json:"description,omitempty"`
	InputSchema   interface{} `json:"input_schema"`
	MaxCharacters int         `json:"max_characters,omitempty"`
}

// AnthropicResponse represents a non-streaming response
type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"` // "message"
	Role         string                  `json:"role"` // "assistant"
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"` // "end_turn", "tool_use", "max_tokens"
	StopSequence *string                 `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}

// AnthropicUsage tracks token usage
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Streaming event types

// AnthropicStreamEvent wraps all SSE event data
type AnthropicStreamEvent struct {
	Type string `json:"type"`
}

// AnthropicMessageStart is the message_start event
type AnthropicMessageStart struct {
	Type    string            `json:"type"` // "message_start"
	Message AnthropicResponse `json:"message"`
}

// AnthropicContentBlockStart is the content_block_start event
type AnthropicContentBlockStart struct {
	Type         string                `json:"type"` // "content_block_start"
	Index        int                   `json:"index"`
	ContentBlock AnthropicContentBlock `json:"content_block"`
}

// AnthropicContentBlockDelta is the content_block_delta event
type AnthropicContentBlockDelta struct {
	Type  string                      `json:"type"` // "content_block_delta"
	Index int                         `json:"index"`
	Delta AnthropicContentBlockDelta2 `json:"delta"`
}

// AnthropicContentBlockDelta2 is the delta payload within content_block_delta
type AnthropicContentBlockDelta2 struct {
	Type        string `json:"type"`                   // "text_delta", "thinking_delta", "input_json_delta"
	Text        string `json:"text,omitempty"`         // for text_delta
	Thinking    string `json:"thinking,omitempty"`     // for thinking_delta
	PartialJSON string `json:"partial_json,omitempty"` // for input_json_delta
}

// AnthropicContentBlockStop is the content_block_stop event
type AnthropicContentBlockStop struct {
	Type  string `json:"type"` // "content_block_stop"
	Index int    `json:"index"`
}

// AnthropicMessageDelta is the message_delta event
type AnthropicMessageDelta struct {
	Type  string `json:"type"` // "message_delta"
	Delta struct {
		StopReason   string  `json:"stop_reason"`
		StopSequence *string `json:"stop_sequence"`
	} `json:"delta"`
	Usage AnthropicUsage `json:"usage"`
}

// AnthropicMessageStop is the message_stop event
type AnthropicMessageStop struct {
	Type string `json:"type"` // "message_stop"
}
