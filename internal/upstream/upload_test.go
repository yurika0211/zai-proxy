package upstream

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setupMockUploadServer creates a mock server and sets uploadBaseURL
func setupMockUploadServer() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, `{"detail":"401 Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "no file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-mock-123",
			"user_id":  "user-1",
			"filename": header.Filename,
			"meta": map[string]interface{}{
				"name":         header.Filename,
				"content_type": "image/png",
				"size":         header.Size,
				"cdn_url":      "https://cdn.example.com/file-mock-123",
			},
		})
	}))

	// Override upload base URL
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	// Store original for cleanup
	t := &testing.T{}
	t.Cleanup(func() {
		uploadBaseURL = origURL
		server.Close()
	})

	return server
}

// restoreUploadURL is called via defer in each test
func restoreUploadURL(orig string) {
	uploadBaseURL = orig
}

// ===== UploadImageFromURL with mock server =====

func TestUploadImageFromURL_Base64PNG_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-123",
			"user_id":  "user-1",
			"filename": "test.png",
			"meta": map[string]interface{}{
				"name":         "test.png",
				"content_type": "image/png",
				"size":         100,
				"cdn_url":      "https://cdn.example.com/file-123",
			},
		})
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	pngData := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))
	dataURL := fmt.Sprintf("data:image/png;base64,%s", pngData)

	file, err := UploadImageFromURL("test-token", dataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	if file.ID != "file-123" {
		t.Errorf("expected file-123, got %s", file.ID)
	}
	if file.Type != "image" {
		t.Errorf("expected image type, got %s", file.Type)
	}
	if file.Status != "uploaded" {
		t.Errorf("expected uploaded status, got %s", file.Status)
	}
}

func TestUploadImageFromURL_Base64JPEG_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-jpg",
			"user_id":  "user-1",
			"filename": "test.jpg",
			"meta": map[string]interface{}{
				"name":         "test.jpg",
				"content_type": "image/jpeg",
				"size":         200,
				"cdn_url":      "https://cdn.example.com/file-jpg",
			},
		})
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	jpegData := base64.StdEncoding.EncodeToString([]byte("fake-jpeg-data"))
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", jpegData)

	file, err := UploadImageFromURL("test-token", dataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	// Filename should end with .jpg
	if !strings.HasSuffix(file.Name, ".jpg") {
		t.Errorf("expected filename ending with .jpg, got %s", file.Name)
	}
}

func TestUploadImageFromURL_Base64GIF_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-gif",
			"user_id":  "user-1",
			"filename": "test.gif",
			"meta": map[string]interface{}{
				"name":         "test.gif",
				"content_type": "image/gif",
				"size":         300,
				"cdn_url":      "https://cdn.example.com/file-gif",
			},
		})
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	gifData := base64.StdEncoding.EncodeToString([]byte("fake-gif-data"))
	dataURL := fmt.Sprintf("data:image/gif;base64,%s", gifData)

	file, err := UploadImageFromURL("test-token", dataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file == nil {
		t.Fatal("expected non-nil file")
	}
}

func TestUploadImageFromURL_Base64WebP_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-webp",
			"user_id":  "user-1",
			"filename": "test.webp",
			"meta": map[string]interface{}{
				"name":         "test.webp",
				"content_type": "image/webp",
				"size":         400,
				"cdn_url":      "https://cdn.example.com/file-webp",
			},
		})
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	webpData := base64.StdEncoding.EncodeToString([]byte("fake-webp-data"))
	dataURL := fmt.Sprintf("data:image/webp;base64,%s", webpData)

	file, err := UploadImageFromURL("test-token", dataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file == nil {
		t.Fatal("expected non-nil file")
	}
}

func TestUploadImageFromURL_UploadServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	pngData := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))
	dataURL := fmt.Sprintf("data:image/png;base64,%s", pngData)

	_, err := UploadImageFromURL("test-token", dataURL)
	if err == nil {
		t.Error("expected error for server 500")
	}
	if !strings.Contains(err.Error(), "upload failed") {
		t.Errorf("expected upload failed error, got: %v", err)
	}
}

func TestUploadImageFromURL_InvalidBase64(t *testing.T) {
	_, err := UploadImageFromURL("test-token", "data:image/png;base64,!!!invalid!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
	if !strings.Contains(err.Error(), "base64") {
		t.Errorf("expected base64 error, got: %v", err)
	}
}

func TestUploadImageFromURL_InvalidBase64Format(t *testing.T) {
	_, err := UploadImageFromURL("test-token", "data:image/png")
	if err == nil {
		t.Error("expected error for invalid base64 format (no comma)")
	}
}

func TestUploadImageFromURL_Base64NoMIME(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-nomime",
			"user_id":  "user-1",
			"filename": "test.png",
			"meta": map[string]interface{}{
				"name":         "test.png",
				"content_type": "image/png",
				"size":         100,
				"cdn_url":      "https://cdn.example.com/file-nomime",
			},
		})
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	pngData := base64.StdEncoding.EncodeToString([]byte("fake-data"))
	dataURL := fmt.Sprintf("data:;base64,%s", pngData)

	file, err := UploadImageFromURL("test-token", dataURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file == nil {
		t.Fatal("expected non-nil file")
	}
}

func TestUploadImageFromURL_URLDownload(t *testing.T) {
	// Test with a URL (not base64) — will fail at HTTP download
	_, err := UploadImageFromURL("test-token", "https://invalid.example.com/nonexistent.png")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// ===== UploadImages =====

func TestUploadImages_EmptyList(t *testing.T) {
	files, err := UploadImages("test-token", []string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestUploadImages_SingleInvalidURL(t *testing.T) {
	files, err := UploadImages("test-token", []string{"https://invalid.example.com/nope.png"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files (invalid URL skipped), got %d", len(files))
	}
}

func TestUploadImages_MockSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-batch",
			"user_id":  "user-1",
			"filename": "test.png",
			"meta": map[string]interface{}{
				"name":         "test.png",
				"content_type": "image/png",
				"size":         100,
				"cdn_url":      "https://cdn.example.com/file-batch",
			},
		})
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	pngData := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))
	dataURL := fmt.Sprintf("data:image/png;base64,%s", pngData)

	files, err := UploadImages("test-token", []string{dataURL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].ID != "file-batch" {
		t.Errorf("expected file-batch, got %s", files[0].ID)
	}
}

func TestUploadImages_MixedSuccessAndFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-ok",
			"user_id":  "user-1",
			"filename": "test.png",
			"meta": map[string]interface{}{
				"name":         "test.png",
				"content_type": "image/png",
				"size":         100,
				"cdn_url":      "https://cdn.example.com/file-ok",
			},
		})
	}))
	defer server.Close()
	origURL := uploadBaseURL
	uploadBaseURL = server.URL
	defer restoreUploadURL(origURL)

	pngData := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))
	dataURL := fmt.Sprintf("data:image/png;base64,%s", pngData)

	// One valid base64 (will succeed with mock), one invalid URL (will fail)
	files, err := UploadImages("test-token", []string{dataURL, "https://invalid.example.com/nope.png"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (one failed), got %d", len(files))
	}
}

// ===== min =====

func TestMin_ALessThanB(t *testing.T) {
	if min(3, 5) != 3 {
		t.Errorf("expected 3, got %d", min(3, 5))
	}
}

func TestMin_BLessThanA(t *testing.T) {
	if min(5, 3) != 3 {
		t.Errorf("expected 3, got %d", min(5, 3))
	}
}

func TestMin_Equal(t *testing.T) {
	if min(4, 4) != 4 {
		t.Errorf("expected 4, got %d", min(4, 4))
	}
}

func TestMin_Zero(t *testing.T) {
	if min(0, 5) != 0 {
		t.Errorf("expected 0, got %d", min(0, 5))
	}
}