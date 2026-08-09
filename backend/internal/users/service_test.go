package users

import (
	"errors"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	tests := map[string]string{
		"+7 999 123-45-67":  "+79991234567",
		"8 (999) 123-45-67": "+79991234567",
		"9991234567":        "+79991234567",
		"+14155552671":      "+14155552671",
	}
	for input, want := range tests {
		got, err := NormalizePhone(input)
		if err != nil {
			t.Fatalf("NormalizePhone(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizePhoneRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "123", "+0 999 123 45 67", "+7 call me"} {
		_, err := NormalizePhone(input)
		var validationError *ValidationError
		if !errors.As(err, &validationError) {
			t.Fatalf("NormalizePhone(%q) error = %v, want ValidationError", input, err)
		}
	}
}
