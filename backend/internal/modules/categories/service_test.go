package categories

import (
	"context"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"gorm.io/gorm"
)

type categoryContextKey struct{}

func TestCategoryServicePassesContextToGORMQueries(t *testing.T) {
	s := testutil.NewStore(t)
	service := NewService(s, nil)
	ctx := context.WithValue(context.Background(), categoryContextKey{}, "request-context")

	callbackName := "categories:test_context"
	var got any
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		got = db.Statement.Context.Value(categoryContextKey{})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.DB.Callback().Query().Remove(callbackName)
	})

	if _, err := service.List(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if got != "request-context" {
		t.Fatalf("gorm context value = %#v", got)
	}
}

func TestCategoryServiceAllowsNilInvalidator(t *testing.T) {
	s := testutil.NewStore(t)
	service := NewService(s, nil)

	category, err := service.Create(context.Background(), 1, categoryRequest{Name: "Bonus", Type: "income"})
	if err != nil {
		t.Fatal(err)
	}
	if category.ID == 0 {
		t.Fatal("expected category to be created")
	}
}
