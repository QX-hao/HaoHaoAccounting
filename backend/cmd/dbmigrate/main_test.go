package main

import (
	"errors"
	"reflect"
	"testing"
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

func TestIsIgnorableMigrationErrorDetectsDuplicateIndex(t *testing.T) {
	if !isIgnorableMigrationError(errors.New("Error 1061 (42000): Duplicate key name 'idx_name'")) {
		t.Fatal("expected duplicate key name to be ignorable")
	}
	if isIgnorableMigrationError(errors.New("syntax error")) {
		t.Fatal("unexpectedly ignored syntax error")
	}
}
