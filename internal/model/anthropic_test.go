package model

import (
	"testing"
)

func TestAnthropicMessageParseContent_StringContent(t *testing.T) {
	msg := &AnthropicMessage{
		Role:    "assistant",
		Content: "Hello, world!",
	}

	text, blocks := msg.ParseContent()
	if text != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", text)
	}
	if len(blocks) != 0 {
		t.Errorf("expected no blocks, got %d", len(blocks))
	}
}

func TestAnthropicMessageParseContent_TextBlock(t *testing.T) {
	msg := &AnthropicMessage{
		Role: "assistant",
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Block text",
			},
		},
	}

	text, blocks := msg.ParseContent()
	if text != "Block text" {
		t.Errorf("expected 'Block text', got %q", text)
	}
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("expected block type 'text', got %q", blocks[0].Type)
	}
}

func TestAnthropicMessageParseContent_MultipleTextBlocks(t *testing.T) {
	msg := &AnthropicMessage{
		Role: "assistant",
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "First ",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Second",
			},
		},
	}

	text, blocks := msg.ParseContent()
	if text != "First Second" {
		t.Errorf("expected 'First Second', got %q", text)
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
}

func TestAnthropicMessageParseContent_MixedBlocks(t *testing.T) {
	msg := &AnthropicMessage{
		Role: "assistant",
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Text content",
			},
			map[string]interface{}{
				"type": "tool_use",
				"id":   "tool-123",
				"name": "calculator",
			},
		},
	}

	text, blocks := msg.ParseContent()
	if text != "Text content" {
		t.Errorf("expected 'Text content', got %q", text)
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
}

func TestAnthropicMessageParseContent_EmptyContent(t *testing.T) {
	msg := &AnthropicMessage{
		Role:    "assistant",
		Content: "",
	}

	text, blocks := msg.ParseContent()
	if text != "" {
		t.Errorf("expected empty string, got %q", text)
	}
	if len(blocks) != 0 {
		t.Errorf("expected no blocks, got %d", len(blocks))
	}
}

func TestAnthropicMessageParseContent_EmptyArray(t *testing.T) {
	msg := &AnthropicMessage{
		Role:    "assistant",
		Content: []interface{}{},
	}

	text, blocks := msg.ParseContent()
	if text != "" {
		t.Errorf("expected empty string, got %q", text)
	}
	if len(blocks) != 0 {
		t.Errorf("expected no blocks, got %d", len(blocks))
	}
}

func TestAnthropicMessageParseContent_InvalidJSON(t *testing.T) {
	msg := &AnthropicMessage{
		Role: "assistant",
		Content: []interface{}{
			"invalid",
		},
	}

	text, blocks := msg.ParseContent()
	if text != "" {
		t.Errorf("expected empty string for invalid JSON, got %q", text)
	}
	if len(blocks) != 0 {
		t.Errorf("expected no blocks for invalid JSON, got %d", len(blocks))
	}
}

func TestAnthropicMessageParseContent_NilContent(t *testing.T) {
	msg := &AnthropicMessage{
		Role:    "assistant",
		Content: nil,
	}

	text, blocks := msg.ParseContent()
	if text != "" {
		t.Errorf("expected empty string for nil content, got %q", text)
	}
	if len(blocks) != 0 {
		t.Errorf("expected no blocks for nil content, got %d", len(blocks))
	}
}

func TestUpstreamDataGetEditContent_Empty(t *testing.T) {
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: "",
		},
	}

	result := data.GetEditContent()
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestUpstreamDataGetEditContent_PlainString(t *testing.T) {
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: "plain text",
		},
	}

	result := data.GetEditContent()
	if result != "plain text" {
		t.Errorf("expected 'plain text', got %q", result)
	}
}

func TestUpstreamDataGetEditContent_JSONString(t *testing.T) {
	jsonStr := `"escaped string"`
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: jsonStr,
		},
	}

	result := data.GetEditContent()
	if result != "escaped string" {
		t.Errorf("expected 'escaped string', got %q", result)
	}
}

func TestUpstreamDataGetEditContent_JSONWithSpecialChars(t *testing.T) {
	jsonStr := `"text with \"quotes\" and \n newlines"`
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: jsonStr,
		},
	}

	result := data.GetEditContent()
	if result != `text with "quotes" and 
 newlines` {
		t.Errorf("expected unescaped string, got %q", result)
	}
}

func TestUpstreamDataGetEditContent_InvalidJSON(t *testing.T) {
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: `"invalid json`,
		},
	}

	result := data.GetEditContent()
	if result != `"invalid json` {
		t.Errorf("expected original string for invalid JSON, got %q", result)
	}
}

func TestUpstreamDataGetEditContent_NotQuotedString(t *testing.T) {
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: "not quoted",
		},
	}

	result := data.GetEditContent()
	if result != "not quoted" {
		t.Errorf("expected 'not quoted', got %q", result)
	}
}

func TestUpstreamDataGetEditContent_ComplexJSON(t *testing.T) {
	// When EditContent is a JSON-encoded string, it should be unescaped
	jsonStr := `"simple string"`
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: jsonStr,
		},
	}

	result := data.GetEditContent()
	expected := "simple string"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUpstreamDataGetEditContent_UnicodeEscapes(t *testing.T) {
	// JSON unmarshal will convert \uXXXX to actual unicode
	jsonStr := `"Hello \u4e16\u754c"`
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: jsonStr,
		},
	}

	result := data.GetEditContent()
	expected := "Hello 世界"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUpstreamDataGetEditContent_EmptyJSONString(t *testing.T) {
	jsonStr := `""`
	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: jsonStr,
		},
	}

	result := data.GetEditContent()
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestUpstreamDataGetEditContent_LongString(t *testing.T) {
	longText := ""
	for i := 0; i < 1000; i++ {
		longText += "a"
	}
	jsonStr := `"` + longText + `"`

	data := &UpstreamData{
		Type: "edit",
		Data: struct {
			DeltaContent string `json:"delta_content"`
			EditContent  string `json:"edit_content"`
			Phase        string `json:"phase"`
			Done         bool   `json:"done"`
		}{
			EditContent: jsonStr,
		},
	}

	result := data.GetEditContent()
	if result != longText {
		t.Errorf("expected long string, got length %d", len(result))
	}
}
