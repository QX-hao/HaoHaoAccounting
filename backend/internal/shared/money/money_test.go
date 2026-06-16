package money

import "testing"

func TestToCentsExactRejectsInvalidAmounts(t *testing.T) {
	for _, amount := range []float64{-1, 1.234} {
		if _, err := ToCentsExact(amount); err == nil {
			t.Fatalf("ToCentsExact(%v) err = nil", amount)
		}
	}
}

func TestToCentsExactAcceptsWholeCents(t *testing.T) {
	cents, err := ToCentsExact(12.34)
	if err != nil {
		t.Fatalf("ToCentsExact: %v", err)
	}
	if cents != 1234 {
		t.Fatalf("cents = %d, want 1234", cents)
	}
}

func TestParseCentsRejectsMoreThanTwoFractionDigits(t *testing.T) {
	if _, err := ParseCents("1.234"); err == nil {
		t.Fatal("expected more than two fraction digits error")
	}
}
