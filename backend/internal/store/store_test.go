package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type storeContextKey struct{}

func TestConfigPoolConfig(t *testing.T) {
	cfg := Config{
		MaxOpenConns:    8,
		MaxIdleConns:    3,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 5 * time.Second,
	}

	got := cfg.PoolConfig()

	if got.MaxOpenConns != cfg.MaxOpenConns ||
		got.MaxIdleConns != cfg.MaxIdleConns ||
		got.ConnMaxLifetime != cfg.ConnMaxLifetime ||
		got.ConnMaxIdleTime != cfg.ConnMaxIdleTime {
		t.Fatalf("PoolConfig() = %#v", got)
	}
}

func TestApplyPoolConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	err = ApplyPoolConfig(db, PoolConfig{
		MaxOpenConns:    7,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ApplyPoolConfig: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("MaxOpenConnections = %d, want 7", got)
	}
}

func TestStoreCloseClosesDatabaseAndIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

	s := &Store{DB: db}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if s.DB != nil {
		t.Fatal("Close did not clear Store.DB")
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected closed database ping error")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestDBWithContextFallsBackForNilContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	s := &Store{DB: db}
	if got := s.DBWithContext(nil).Statement.Context; got == nil {
		t.Fatal("expected non-nil gorm context")
	}
}

func TestTransactionCommitsOnNilError(t *testing.T) {
	s := newSQLiteStore(t)

	err := s.Transaction(context.Background(), func(tx *gorm.DB) error {
		return tx.Create(&models.User{Username: "committed", PasswordHash: "hash"}).Error
	})
	if err != nil {
		t.Fatalf("Transaction: %v", err)
	}

	var count int64
	if err := s.DB.Model(&models.User{}).Where("username = ?", "committed").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("committed user count = %d, want 1", count)
	}
}

func TestTransactionRollsBackOnError(t *testing.T) {
	s := newSQLiteStore(t)
	wantErr := errors.New("rollback")

	err := s.Transaction(context.Background(), func(tx *gorm.DB) error {
		if err := tx.Create(&models.User{Username: "rolled-back", PasswordHash: "hash"}).Error; err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Transaction error = %v, want %v", err, wantErr)
	}

	var count int64
	if err := s.DB.Model(&models.User{}).Where("username = ?", "rolled-back").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled back user count = %d, want 0", count)
	}
}

func TestTransactionPassesContextToGORM(t *testing.T) {
	s := newSQLiteStore(t)
	ctx := context.WithValue(context.Background(), storeContextKey{}, "request-context")

	callbackName := "store:test_transaction_context"
	var got any
	if err := s.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(db *gorm.DB) {
		got = db.Statement.Context.Value(storeContextKey{})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.DB.Callback().Create().Remove(callbackName)
	})

	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		return tx.Create(&models.User{Username: "ctx-user", PasswordHash: "hash"}).Error
	}); err != nil {
		t.Fatalf("Transaction: %v", err)
	}
	if got != "request-context" {
		t.Fatalf("gorm context value = %#v", got)
	}
}

func newSQLiteStore(t *testing.T) *Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return &Store{DB: db}
}
