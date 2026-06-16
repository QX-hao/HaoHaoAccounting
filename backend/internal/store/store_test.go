package store

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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
