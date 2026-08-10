package repository

import (
	"testing"

	"swap-chain/modules/admin/model"
)

func TestCanApplyTransition(t *testing.T) {
	tests := []struct {
		name         string
		chainStatus  string
		itemStatus   string
		current      string
		target       string
		wantAccepted bool
	}{
		{
			name:         "accepted chain",
			chainStatus:  model.ChainAccepted,
			itemStatus:   "LOCKED",
			current:      model.DeliveryAtPVZ,
			target:       model.DeliveryInTransit,
			wantAccepted: true,
		},
		{
			name:         "completed retry",
			chainStatus:  model.ChainCompleted,
			itemStatus:   "LOCKED",
			current:      model.DeliveryReceived,
			target:       model.DeliveryReceived,
			wantAccepted: true,
		},
		{
			name:        "unlocked item",
			chainStatus: model.ChainAccepted,
			itemStatus:  "MATCHING",
			current:     model.DeliveryAtPVZ,
			target:      model.DeliveryInTransit,
		},
		{
			name:        "completed chain mutation",
			chainStatus: model.ChainCompleted,
			itemStatus:  "LOCKED",
			current:     model.DeliveryInTransit,
			target:      model.DeliveryReceived,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := canApplyTransition(test.chainStatus, test.itemStatus, test.current, test.target)
			if got != test.wantAccepted {
				t.Fatalf("canApplyTransition() = %t, want %t", got, test.wantAccepted)
			}
		})
	}
}
