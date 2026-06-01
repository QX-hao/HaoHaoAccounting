package services

import "testing"

func TestParseNaturalLedgerText(t *testing.T) {
	result := ParseNaturalLedgerText("昨天微信午饭35.5")

	if result.Type != "expense" {
		t.Fatalf("type = %q", result.Type)
	}
	if result.Amount != 35.5 {
		t.Fatalf("amount = %v", result.Amount)
	}
	if result.Category != "餐饮" {
		t.Fatalf("category = %q", result.Category)
	}
	if result.Account != "微信" {
		t.Fatalf("account = %q", result.Account)
	}
	if result.Confidence <= 0.8 {
		t.Fatalf("confidence = %v", result.Confidence)
	}
}
