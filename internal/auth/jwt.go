package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

type JWTPayload struct {
	ID string `json:"id"`
}

func DecodeJWTPayload(token string) (*JWTPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, nil
	}

	payload := parts[1]
	// Add padding if needed
	if padding := 4 - len(payload)%4; padding != 4 {
		payload += strings.Repeat("=", padding)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
	}

	var result JWTPayload
	if err := json.Unmarshal(decoded, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
