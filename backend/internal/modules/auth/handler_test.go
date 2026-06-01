package auth

import (
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
)

func TestEnsureBootstrapAdminCreatesHashedPasswordUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "secret-password")
	t.Setenv("ADMIN_NAME", "管理员")

	s := testutil.NewStore(t)
	if err := NewHandler(s).EnsureBootstrapAdmin(); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	var user models.User
	if err := s.DB.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	if user.PasswordHash == "" || user.PasswordHash == "secret-password" {
		t.Fatalf("password was not hashed: %q", user.PasswordHash)
	}
	if !verifyPassword(user.PasswordHash, "secret-password") {
		t.Fatal("hashed password does not verify")
	}
}
