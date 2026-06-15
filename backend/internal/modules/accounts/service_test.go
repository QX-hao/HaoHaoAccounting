package accounts

import (
	"context"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"gorm.io/gorm"
)

type accountContextKey struct{}

type recordingInvalidator struct {
	ctx    context.Context
	userID uint
}

func (r *recordingInvalidator) InvalidateUser(ctx context.Context, userID uint) {
	r.ctx = ctx
	r.userID = userID
}

func (r *recordingInvalidator) err() error {
	if r.ctx == nil {
		return nil
	}
	return r.ctx.Err()
}

func TestServicePassesContextToGORMQueries(t *testing.T) {
	s := testutil.NewStore(t)
	service := NewService(s, nil)
	ctx := context.WithValue(context.Background(), accountContextKey{}, "request-context")

	if err := s.DB.Create(&models.Account{UserID: 1, Name: "Cash", Type: "cash"}).Error; err != nil {
		t.Fatal(err)
	}

	callbackName := "accounts:test_context"
	var got any
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		got = db.Statement.Context.Value(accountContextKey{})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.DB.Callback().Query().Remove(callbackName)
	})

	if _, err := service.List(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got != "request-context" {
		t.Fatalf("gorm context value = %#v", got)
	}
}

func TestServicePassesNonCanceledContextToInvalidator(t *testing.T) {
	s := testutil.NewStore(t)
	invalidator := &recordingInvalidator{}
	service := NewService(s, invalidator)
	baseCtx := context.WithValue(context.Background(), accountContextKey{}, "request-context")
	requestCtx, cancel := context.WithCancel(baseCtx)
	defer cancel()

	callbackName := "accounts:test_cancel_after_create"
	if err := s.DB.Callback().Create().After("gorm:create").Register(callbackName, func(*gorm.DB) {
		cancel()
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.DB.Callback().Create().Remove(callbackName)
	})

	account, err := service.Create(requestCtx, 7, accountRequest{Name: "Cash", Type: "cash"})
	if err != nil {
		t.Fatal(err)
	}
	if account.ID == 0 {
		t.Fatal("expected account to be created")
	}
	if invalidator.userID != 7 {
		t.Fatalf("invalidated user id = %d", invalidator.userID)
	}
	if got := invalidator.ctx.Value(accountContextKey{}); got != "request-context" {
		t.Fatalf("invalidator context value = %#v", got)
	}
	if err := invalidator.err(); err != nil {
		t.Fatalf("invalidator context error = %v", err)
	}
}

var _ transactions.CacheInvalidator = (*recordingInvalidator)(nil)
