package version

import (
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"zai-proxy/internal/logger"
)

var (
	feVersion   string
	versionLock sync.RWMutex
	httpClient  = &http.Client{Timeout: 10 * time.Second}
)

// SetHTTPClient allows tests to inject a mock HTTP client
func SetHTTPClient(client *http.Client) {
	httpClient = client
}

func GetFeVersion() string {
	versionLock.RLock()
	defer versionLock.RUnlock()
	return feVersion
}

func fetchFeVersion() {
	resp, err := httpClient.Get("https://chat.z.ai/")
	if err != nil {
		logger.LogError("Failed to fetch fe version: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.LogError("Failed to read fe version response: %v", err)
		return
	}

	re := regexp.MustCompile(`prod-fe-[\.\d]+`)
	match := re.FindString(string(body))
	if match != "" {
		versionLock.Lock()
		feVersion = match
		versionLock.Unlock()
		logger.LogInfo("Updated fe version: %s", match)
	}
}

func StartVersionUpdater() {
	fetchFeVersion()

	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			fetchFeVersion()
		}
	}()
}
