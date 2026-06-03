package main

import (
	"reflect"
	"testing"
)

func TestCorsAllowOriginsDefaultsToLocalWeb(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "")

	got := corsAllowOrigins()
	want := []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("corsAllowOrigins() = %#v, want %#v", got, want)
	}
}

func TestCorsAllowOriginsReadsCommaSeparatedEnv(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", " https://app.example.com,https://admin.example.com ,, ")

	got := corsAllowOrigins()
	want := []string{"https://app.example.com", "https://admin.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("corsAllowOrigins() = %#v, want %#v", got, want)
	}
}
