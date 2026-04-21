package upstream

import (
	"testing"

	"zai-proxy/internal/model"
)

func TestExtractLatestUserContent_WithComplexContent(t *testing.T) {
	messages := []model.Message{
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "First message"},
			},
		},
		{
			Role: "assistant",
			Content: "Response",
		},
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Second message"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img.png",
					},
				},
			},
		},
	}

	result := ExtractLatestUserContent(messages)
	if result != "Second message" {
		t.Errorf("expected 'Second message', got %q", result)
	}
}

func TestExtractLatestUserContent_OnlySystemMessages(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "System 1"},
		{Role: "system", Content: "System 2"},
	}

	result := ExtractLatestUserContent(messages)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractAllImageURLs_MultipleImagesInOneMessage(t *testing.T) {
	messages := []model.Message{
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Multiple images"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img1.png",
					},
				},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img2.png",
					},
				},
			},
		},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
	if urls[0] != "https://example.com/img1.png" {
		t.Errorf("expected first URL https://example.com/img1.png, got %s", urls[0])
	}
	if urls[1] != "https://example.com/img2.png" {
		t.Errorf("expected second URL https://example.com/img2.png, got %s", urls[1])
	}
}

func TestExtractAllImageURLs_MixedContent(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "System message"},
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Text"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img1.png",
					},
				},
			},
		},
		{Role: "assistant", Content: "Response"},
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img2.png",
					},
				},
			},
		},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
}

func TestExtractAllImageURLs_StringContent(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Just a string"},
		{Role: "assistant", Content: "Another string"},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 0 {
		t.Errorf("expected no URLs, got %v", urls)
	}
}

func TestExtractAllImageURLs_EmptyMessages(t *testing.T) {
	urls := ExtractAllImageURLs([]model.Message{})
	if len(urls) != 0 {
		t.Errorf("expected no URLs, got %v", urls)
	}
}

func TestExtractLatestUserContent_MultipleUserMessages(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "First"},
		{Role: "user", Content: "Second"},
		{Role: "user", Content: "Third"},
	}

	result := ExtractLatestUserContent(messages)
	if result != "Third" {
		t.Errorf("expected 'Third', got %q", result)
	}
}

func TestExtractLatestUserContent_UserAfterAssistant(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "First"},
		{Role: "assistant", Content: "Response"},
		{Role: "assistant", Content: "Another response"},
		{Role: "user", Content: "Second"},
	}

	result := ExtractLatestUserContent(messages)
	if result != "Second" {
		t.Errorf("expected 'Second', got %q", result)
	}
}

func TestExtractAllImageURLs_OnlyTextMessages(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Text 1"},
		{Role: "assistant", Content: "Text 2"},
		{Role: "user", Content: "Text 3"},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 0 {
		t.Errorf("expected no URLs, got %v", urls)
	}
}

func TestExtractAllImageURLs_ComplexStructure(t *testing.T) {
	messages := []model.Message{
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Check these"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/a.jpg",
					},
				},
				map[string]interface{}{"type": "text", "text": "And this"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/b.jpg",
					},
				},
				map[string]interface{}{"type": "text", "text": "Done"},
			},
		},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
	if urls[0] != "https://example.com/a.jpg" || urls[1] != "https://example.com/b.jpg" {
		t.Errorf("unexpected URLs: %v", urls)
	}
}

func TestExtractLatestUserContent_ContentWithSpecialChars(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Hello\nWorld\t!@#$%"},
	}

	result := ExtractLatestUserContent(messages)
	if result != "Hello\nWorld\t!@#$%" {
		t.Errorf("expected special chars preserved, got %q", result)
	}
}

func TestExtractLatestUserContent_LongContent(t *testing.T) {
	longText := ""
	for i := 0; i < 1000; i++ {
		longText += "a"
	}

	messages := []model.Message{
		{Role: "user", Content: longText},
	}

	result := ExtractLatestUserContent(messages)
	if result != longText {
		t.Errorf("expected long content preserved, got length %d", len(result))
	}
}

func TestExtractAllImageURLs_DuplicateURLs(t *testing.T) {
	messages := []model.Message{
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/same.png",
					},
				},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/same.png",
					},
				},
			},
		},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs (including duplicates), got %d", len(urls))
	}
	if urls[0] != urls[1] {
		t.Error("expected duplicate URLs to be preserved")
	}
}
