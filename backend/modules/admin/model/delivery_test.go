package model

import (
	"errors"
	"testing"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		target      string
		wantAllowed bool
	}{
		{name: "receive at pickup point", current: DeliveryAwaitingPVZ, target: DeliveryAtPVZ, wantAllowed: true},
		{name: "dispatch", current: DeliveryAtPVZ, target: DeliveryInTransit, wantAllowed: true},
		{name: "confirm receipt", current: DeliveryInTransit, target: DeliveryReceived, wantAllowed: true},
		{name: "receipt retry", current: DeliveryReceived, target: DeliveryReceived, wantAllowed: true},
		{name: "idempotent retry", current: DeliveryAtPVZ, target: DeliveryAtPVZ, wantAllowed: true},
		{name: "cannot skip receipt", current: DeliveryAwaitingPVZ, target: DeliveryInTransit, wantAllowed: false},
		{name: "cannot move backwards", current: DeliveryInTransit, target: DeliveryAtPVZ, wantAllowed: false},
		{name: "cannot skip delivery", current: DeliveryAtPVZ, target: DeliveryReceived, wantAllowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanTransition(test.current, test.target); got != test.wantAllowed {
				t.Fatalf("CanTransition(%q, %q) = %t, want %t", test.current, test.target, got, test.wantAllowed)
			}
		})
	}
}

func TestValidateTargetStatus(t *testing.T) {
	for _, status := range []string{DeliveryAtPVZ, DeliveryInTransit, DeliveryReceived} {
		if err := ValidateTargetStatus(status); err != nil {
			t.Fatalf("ValidateTargetStatus(%q) error = %v", status, err)
		}
	}
	var validationError *ValidationError
	if err := ValidateTargetStatus(DeliveryAwaitingPVZ); !errors.As(err, &validationError) || validationError.Field != "status" {
		t.Fatalf("ValidateTargetStatus() error = %v", err)
	}
}
