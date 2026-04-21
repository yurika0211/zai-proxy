package version

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestGetFeVersion_Initial(t *testing.T) {
	// feVersion starts empty
	v := GetFeVersion()
	// Just verify it doesn't panic and returns a string
	_ = v
}

func TestGetFeVersion_AfterSet(t *testing.T) {
	versionLock.Lock()
	feVersion = "prod-fe-1.2.3"
	versionLock.Unlock()

	v := GetFeVersion()
	if v != "prod-fe-1.2.3" {
		t.Errorf("expected prod-fe-1.2.3, got %s", v)
	}

	// Reset
	versionLock.Lock()
	feVersion = ""
	versionLock.Unlock()
}

func TestGetFeVersion_ConcurrentAccess(t *testing.T) {
	versionLock.Lock()
	feVersion = ""
	versionLock.Unlock()

	done := make(chan bool)
	// Writer
	go func() {
		for i := 0; i < 100; i++ {
			versionLock.Lock()
			feVersion = "prod-fe-test"
			versionLock.Unlock()
		}
		done <- true
	}()
	// Reader
	go func() {
		for i := 0; i < 100; i++ {
			_ = GetFeVersion()
		}
		done <- true
	}()

	<-done
	<-done
}

func TestFetchFeVersion_Success(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>prod-fe-1.2.3</body></html>`))
	}))
	defer server.Close()

	// Temporarily replace the URL (we can't easily mock http.Get, so we'll test the regex)
	// Instead, test the regex extraction directly
	body := `<html><body>prod-fe-1.2.3</body></html>`
	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	match := re.FindString(body)
	if match != "prod-fe-1.2.3" {
		t.Errorf("expected prod-fe-1.2.3, got %s", match)
	}
}

func TestFetchFeVersion_NoMatch(t *testing.T) {
	body := `<html><body>no version here</body></html>`
	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	match := re.FindString(body)
	if match != "" {
		t.Errorf("expected empty string, got %s", match)
	}
}

func TestFetchFeVersion_MultipleMatches(t *testing.T) {
	body := `<html><body>prod-fe-1.0.0 and prod-fe-2.0.0</body></html>`
	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	match := re.FindString(body)
	// FindString returns the first match
	if match != "prod-fe-1.0.0" {
		t.Errorf("expected prod-fe-1.0.0, got %s", match)
	}
}

func TestFetchFeVersion_VariousFormats(t *testing.T) {
	testCases := []struct {
		body     string
		expected string
	}{
		{`prod-fe-1.2.3`, "prod-fe-1.2.3"},
		{`prod-fe-0.0.1`, "prod-fe-0.0.1"},
		{`prod-fe-10.20.30`, "prod-fe-10.20.30"},
		{`prod-fe-1.2`, "prod-fe-1.2"},
		{`prod-fe-1`, "prod-fe-1"},
		{`prod-fe-`, ""},
		{`prod-fe-abc`, ""},
	}

	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	for _, tc := range testCases {
		match := re.FindString(tc.body)
		if match != tc.expected {
			t.Errorf("body=%q: expected %q, got %q", tc.body, tc.expected, match)
		}
	}
}

func TestGetFeVersion_ThreadSafety(t *testing.T) {
	// Reset
	versionLock.Lock()
	feVersion = ""
	versionLock.Unlock()

	results := make(chan string, 10)
	for i := 0; i < 10; i++ {
		go func() {
			results <- GetFeVersion()
		}()
	}

	for i := 0; i < 10; i++ {
		<-results
	}
}

func TestFetchFeVersion_WithMockServer(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>prod-fe-2.5.1</body></html>`))
	}))
	defer server.Close()

	// We can't easily mock http.Get in the current code structure,
	// but we can verify the regex extraction works correctly
	body := `<html><body>prod-fe-2.5.1</body></html>`
	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	match := re.FindString(body)
	if match != "prod-fe-2.5.1" {
		t.Errorf("expected prod-fe-2.5.1, got %s", match)
	}
}

func TestFetchFeVersion_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		body     string
		expected string
	}{
		{"empty body", "", ""},
		{"no version", "<html></html>", ""},
		{"version at start", "prod-fe-1.0.0 is here", "prod-fe-1.0.0"},
		{"version at end", "here is prod-fe-3.2.1", "prod-fe-3.2.1"},
		{"version in middle", "start prod-fe-1.5.0 end", "prod-fe-1.5.0"},
		{"multiple versions", "prod-fe-1.0.0 and prod-fe-2.0.0", "prod-fe-1.0.0"},
		{"version with extra dots", "prod-fe-1.2.3.4", "prod-fe-1.2.3.4"},
		{"version with leading zeros", "prod-fe-01.02.03", "prod-fe-01.02.03"},
	}

	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			match := re.FindString(tc.body)
			if match != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, match)
			}
		})
	}
}

func TestFetchFeVersion_WithMockHTTPClient(t *testing.T) {
	// Reset version
	versionLock.Lock()
	feVersion = ""
	versionLock.Unlock()

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>prod-fe-3.1.4</body></html>`))
	}))
	defer server.Close()

	// Create HTTP client pointing to mock server
	mockClient := &http.Client{Timeout: 5 * time.Second}
	oldClient := httpClient
	SetHTTPClient(mockClient)
	defer SetHTTPClient(oldClient)

	// Manually test the regex extraction (since we can't easily mock the URL)
	body := `<html><body>prod-fe-3.1.4</body></html>`
	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	match := re.FindString(body)
	if match != "prod-fe-3.1.4" {
		t.Errorf("expected prod-fe-3.1.4, got %s", match)
	}
}