package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type AnonymousAuthResponse struct {
	Token string `json:"token"`
}

var (
	anonymousTokenCache     string
	anonymousTokenExpireAt  time.Time
	anonymousTokenMu        sync.Mutex
	anonymousTokenTTL       = 50 * time.Minute // tokens typically last ~1h
)

// GetAnonymousToken 从 z.ai 获取匿名 token，带缓存
func GetAnonymousToken() (string, error) {
	anonymousTokenMu.Lock()
	defer anonymousTokenMu.Unlock()

	if anonymousTokenCache != "" && time.Now().Before(anonymousTokenExpireAt) {
		return anonymousTokenCache, nil
	}

	resp, err := http.Get("https://chat.z.ai/api/v1/auths/")
	if err != nil {
		// Return stale token if available rather than failing completely
		if anonymousTokenCache != "" {
			return anonymousTokenCache, nil
		}
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if anonymousTokenCache != "" {
			return anonymousTokenCache, nil
		}
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var authResp AnonymousAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		if anonymousTokenCache != "" {
			return anonymousTokenCache, nil
		}
		return "", err
	}

	if authResp.Token == "" {
		return "", fmt.Errorf("empty token from anonymous auth")
	}

	anonymousTokenCache = authResp.Token
	anonymousTokenExpireAt = time.Now().Add(anonymousTokenTTL)

	return authResp.Token, nil
}