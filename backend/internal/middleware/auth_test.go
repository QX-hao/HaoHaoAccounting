package middleware

import (
	"strings"
	"testing"
)

func TestJWTSecretIsRequiredAndStrong(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := JWTSecret(); err == nil {
		t.Fatal("expected missing secret error")
	}

	t.Setenv("JWT_SECRET", "short")
	if _, err := JWTSecret(); err == nil {
		t.Fatal("expected short secret error")
	}
}

func TestBuildAndParseToken(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("x", 32))

	token, err := BuildToken(42)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	userID, err := ParseToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if userID != 42 {
		t.Fatalf("userID = %d", userID)
	}
}
