package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

func hmacSha256Hex(key []byte, data string) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func GenerateSignature(userID, requestID, userContent string, timestamp int64) string {
	requestInfo := fmt.Sprintf("requestId,%s,timestamp,%d,user_id,%s", requestID, timestamp, userID)
	contentBase64 := base64.StdEncoding.EncodeToString([]byte(userContent))
	signData := fmt.Sprintf("%s|%s|%d", requestInfo, contentBase64, timestamp)

	period := timestamp / (5 * 60 * 1000)
	firstHmac := hmacSha256Hex([]byte("key-@@@@)))()((9))-xxxx&&&%%%%%"), fmt.Sprintf("%d", period))
	signature := hmacSha256Hex([]byte(firstHmac), signData)

	return signature
}
