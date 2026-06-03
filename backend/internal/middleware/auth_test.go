package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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

func TestRequireAuthRejectsRevokedToken(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("x", 32))
	gin.SetMode(gin.TestMode)

	token, err := BuildToken(42)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	router := gin.New()
	router.GET("/private", RequireAuthWithRevocation(staticRevocationChecker{revoked: true}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

type staticRevocationChecker struct {
	revoked bool
}

func (c staticRevocationChecker) IsTokenRevoked(context.Context, string) (bool, error) {
	return c.revoked, nil
}
