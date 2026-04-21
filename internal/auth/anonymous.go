package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type AnonymousAuthResponse struct {
	Token string `json:"token"`
}

// GetAnonymousToken 从 z.ai 获取匿名 token
func GetAnonymousToken() (string, error) {
	resp, err := http.Get("https://chat.z.ai/api/v1/auths/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var authResp AnonymousAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	return authResp.Token, nil
}
