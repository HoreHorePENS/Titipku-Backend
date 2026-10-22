package domain

import (
	"strings"
	"testing"
)

func TestNormalizeWhatsApp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "local zero", input: "0812-3456-7890", want: "6281234567890", ok: true},
		{name: "country code", input: "+62 812 3456 7890", want: "6281234567890", ok: true},
		{name: "without prefix", input: "81234567890", want: "6281234567890", ok: true},
		{name: "invalid too short", input: "0812", ok: false},
		{name: "invalid prefix", input: "123456789012", ok: false},
		{name: "empty", input: "", ok: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, ok := NormalizeWhatsApp(test.input)
			if ok != test.ok {
				t.Fatalf("NormalizeWhatsApp(%q) ok = %v, want %v", test.input, ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("NormalizeWhatsApp(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestGeneratePublicToken(t *testing.T) {
	t.Parallel()

	token, err := GeneratePublicToken()
	if err != nil {
		t.Fatalf("GeneratePublicToken() error = %v", err)
	}
	if len(token) < 40 {
		t.Fatalf("GeneratePublicToken() length = %d, want at least 40", len(token))
	}
	if strings.ContainsAny(token, "+/=") {
		t.Fatalf("GeneratePublicToken() = %q, want URL-safe characters", token)
	}

	second, err := GeneratePublicToken()
	if err != nil {
		t.Fatalf("GeneratePublicToken() second error = %v", err)
	}
	if token == second {
		t.Fatal("GeneratePublicToken() returned the same token twice")
	}
}

func TestValidationBoundaries(t *testing.T) {
	t.Parallel()

	if !IsValidEmail("owner@example.com") {
		t.Fatal("expected valid email")
	}
	if IsValidEmail("not-an-email") {
		t.Fatal("expected invalid email")
	}
	if IsValidPassword("short") {
		t.Fatal("expected short password to be invalid")
	}
	if !IsValidPassword("rahasia123") {
		t.Fatal("expected valid password")
	}
	if !IsValidShelfLife(3) {
		t.Fatal("expected valid shelf life")
	}
	if IsValidShelfLife(0) {
		t.Fatal("expected zero shelf life to be invalid")
	}
	if !IsValidPaymentMethod("qris") {
		t.Fatal("expected valid payment method")
	}
	if IsValidPaymentMethod("cheque") {
		t.Fatal("expected unsupported payment method to be invalid")
	}
}
