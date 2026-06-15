package main

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
)

func TestSplitSQLStatementsSkipsBlankAndCommentLines(t *testing.T) {
	got := splitSQLStatements(`
-- first table
CREATE TABLE users (id BIGINT PRIMARY KEY);

-- second table
CREATE TABLE accounts (id BIGINT PRIMARY KEY);
`)
	want := []string{
		"CREATE TABLE users (id BIGINT PRIMARY KEY)",
		"CREATE TABLE accounts (id BIGINT PRIMARY KEY)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitSQLStatements() = %#v, want %#v", got, want)
	}
}

func TestNormalizeDriverAliasesPostgres(t *testing.T) {
	if got := normalizeDriver(" pgsql "); got != "postgres" {
		t.Fatalf("normalizeDriver() = %q, want postgres", got)
	}
}

func TestMigrationDatabaseConfigRequiresExplicitDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")

	_, err := migrationDatabaseConfig(config.Config{
		Database: config.DatabaseConfig{Driver: "postgres", DSN: "default-dsn"},
	})
	if err == nil {
		t.Fatal("expected explicit DB_DSN requirement")
	}
}

func TestMigrationDatabaseConfigNormalizesDriver(t *testing.T) {
	t.Setenv("DB_DSN", "postgres-dsn")

	cfg, err := migrationDatabaseConfig(config.Config{
		Database: config.DatabaseConfig{
			Driver:          " pgsql ",
			DSN:             "ignored-default",
			MaxOpenConns:    9,
			MaxIdleConns:    4,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: 30 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("migrationDatabaseConfig: %v", err)
	}
	if cfg.Driver != "postgres" {
		t.Fatalf("Driver = %q", cfg.Driver)
	}
	if cfg.DSN != "postgres-dsn" {
		t.Fatalf("DSN = %q", cfg.DSN)
	}
	if cfg.MaxOpenConns != 9 ||
		cfg.MaxIdleConns != 4 ||
		cfg.ConnMaxLifetime != time.Minute ||
		cfg.ConnMaxIdleTime != 30*time.Second {
		t.Fatalf("Database pool config = %#v", cfg)
	}
}

func TestIsIgnorableMigrationErrorDetectsDuplicateIndex(t *testing.T) {
	if !isIgnorableMigrationError(errors.New("Error 1061 (42000): Duplicate key name 'idx_name'")) {
		t.Fatal("expected duplicate key name to be ignorable")
	}
	if isIgnorableMigrationError(errors.New("syntax error")) {
		t.Fatal("unexpectedly ignored syntax error")
	}
}
