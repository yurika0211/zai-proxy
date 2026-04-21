package auth

import (
	"testing"
)

func TestGenerateSignature_Deterministic(t *testing.T) {
	sig1 := GenerateSignature("user1", "req1", "hello", 1000)
	sig2 := GenerateSignature("user1", "req1", "hello", 1000)
	if sig1 != sig2 {
		t.Errorf("signature should be deterministic, got %s vs %s", sig1, sig2)
	}
}

func TestGenerateSignature_DifferentInputs(t *testing.T) {
	sig1 := GenerateSignature("user1", "req1", "hello", 1000)
	sig2 := GenerateSignature("user2", "req1", "hello", 1000)
	if sig1 == sig2 {
		t.Error("different userIDs should produce different signatures")
	}

	sig3 := GenerateSignature("user1", "req2", "hello", 1000)
	if sig1 == sig3 {
		t.Error("different requestIDs should produce different signatures")
	}

	sig4 := GenerateSignature("user1", "req1", "world", 1000)
	if sig1 == sig4 {
		t.Error("different content should produce different signatures")
	}

	sig5 := GenerateSignature("user1", "req1", "hello", 2000)
	if sig1 == sig5 {
		t.Error("different timestamps should produce different signatures")
	}
}

func TestGenerateSignature_NonEmpty(t *testing.T) {
	sig := GenerateSignature("user1", "req1", "content", 1234567890)
	if sig == "" {
		t.Error("signature should not be empty")
	}
}

func TestGenerateSignature_EmptyContent(t *testing.T) {
	sig := GenerateSignature("user1", "req1", "", 1000)
	if sig == "" {
		t.Error("signature should not be empty even with empty content")
	}
}