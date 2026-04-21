package upstream

import (
	"testing"

	"zai-proxy/internal/model"
)

func TestExtractLatestUserContent_WithUserMessage(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "How are you?"},
	}

	result := ExtractLatestUserContent(messages)
	if result != "How are you?" {
		t.Errorf("expected 'How are you?', got %q", result)
	}
}

func TestExtractLatestUserContent_NoUserMessage(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "assistant", Content: "Hi there"},
	}

	result := ExtractLatestUserContent(messages)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractLatestUserContent_EmptyMessages(t *testing.T) {
	result := ExtractLatestUserContent([]model.Message{})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractLatestUserContent_SingleUserMessage(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Only message"},
	}

	result := ExtractLatestUserContent(messages)
	if result != "Only message" {
		t.Errorf("expected 'Only message', got %q", result)
	}
}

func TestExtractAllImageURLs_NoImages(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Just text"},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 0 {
		t.Errorf("expected no URLs, got %v", urls)
	}
}

func TestExtractAllImageURLs_WithImages(t *testing.T) {
	messages := []model.Message{
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Look at this"},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img.png",
					},
				},
			},
		},
	}

	urls := ExtractAllImageURLs(messages)
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	if urls[0] != "https://example.com/img.png" {
		t.Errorf("expected https://example.com/img.png, got %s", urls[0])
	}
}

func TestExtractAllImageURLs_MultipleMessages(t *testing.T) {
	messages := []model.Message{
		{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "https://example.com/img1.png",
					},
				},
			},
		},
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