package repository

import (
	"database/sql"
	"testing"
	"time"

	"swap-chain/modules/admin/model"
	"swap-chain/shared/db"
)

func TestNewPostgreSQLRequiresDatabase(t *testing.T) {
	if _, err := NewPostgreSQL(nil); err == nil {
		t.Fatal("NewPostgreSQL(nil) error = nil")
	}
	if _, err := NewPostgreSQL(&sql.DB{}); err != nil {
		t.Fatalf("NewPostgreSQL() error = %v", err)
	}
}

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

func TestDeliveryMappings(t *testing.T) {
	updatedAt := time.Unix(100, 0).UTC()
	listed := mapListedDelivery(db.ListAdminDeliveriesRow{
		ID: 1, ChainID: 2, ItemID: 3, ItemTitle: "Телефон",
		SenderID: 4, SenderUsername: "Аня", RecipientID: 5, RecipientUsername: "Борис",
		DeliveryStatus: model.DeliveryAtPVZ, DeliveryUpdatedAt: updatedAt,
	})
	loaded := mapDelivery(db.GetAdminDeliveryRow{
		ID: 1, ChainID: 2, ItemID: 3, ItemTitle: "Телефон",
		SenderID: 4, SenderUsername: "Аня", RecipientID: 5, RecipientUsername: "Борис",
		DeliveryStatus: model.DeliveryAtPVZ, DeliveryUpdatedAt: updatedAt,
	})
	if listed != loaded || listed.Status != model.DeliveryAtPVZ || !listed.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("delivery mappings = %+v / %+v", listed, loaded)
	}
}
