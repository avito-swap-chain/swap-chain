package repository

import (
	"errors"
	"testing"
	"time"

	"swap-chain/modules/chat/model"
	"swap-chain/shared/db"
)

func TestNewPostgreSQLRequiresDatabase(t *testing.T) {
	if _, err := NewPostgreSQL(nil); err == nil {
		t.Fatal("NewPostgreSQL(nil) error = nil")
	}
}

func TestChatMessageMappings(t *testing.T) {
	createdAt := time.Unix(100, 0).UTC()
	row := db.GetChatMessageRow{
		ID: 7, ItemID: 9, OriginChainID: 8,
		SenderUserID: 1, SenderUsername: "Аня",
		RecipientUserID: 2, RecipientUsername: "Борис",
		ClientMessageID: "client-1", MessageText: "Привет", CreatedAt: createdAt,
	}
	got := mapChatMessage(row)
	if got.ID != 7 || got.ItemID != 9 || got.OriginChainID != 8 || got.Sender.ID != 1 || got.Recipient.ID != 2 ||
		got.ClientMessageID != "client-1" || got.Text != "Привет" || !got.CreatedAt.Equal(createdAt) {
		t.Fatalf("mapChatMessage() = %+v", got)
	}

	idempotent := mapIdempotentMessage(db.GetChatMessageByClientIDRow{
		ID: row.ID, ItemID: row.ItemID, OriginChainID: row.OriginChainID,
		SenderUserID: row.SenderUserID, SenderUsername: row.SenderUsername,
		RecipientUserID: row.RecipientUserID, RecipientUsername: row.RecipientUsername,
		ClientMessageID: row.ClientMessageID, MessageText: row.MessageText, CreatedAt: row.CreatedAt,
	})
	listed := mapListedMessage(db.ListChatMessagesRow{
		ID: row.ID, ItemID: row.ItemID, OriginChainID: row.OriginChainID,
		SenderUserID: row.SenderUserID, SenderUsername: row.SenderUsername,
		RecipientUserID: row.RecipientUserID, RecipientUsername: row.RecipientUsername,
		ClientMessageID: row.ClientMessageID, MessageText: row.MessageText, CreatedAt: row.CreatedAt,
	})
	if idempotent != got || listed != got {
		t.Fatalf("mapping mismatch: created=%+v idempotent=%+v listed=%+v", got, idempotent, listed)
	}
}

func TestValidateAccess(t *testing.T) {
	tests := []struct {
		name                   string
		actorParticipant       bool
		counterpartParticipant bool
		neighbor               bool
		wantErr                error
	}{
		{name: "neighboring participants", actorParticipant: true, counterpartParticipant: true, neighbor: true},
		{name: "outsider", counterpartParticipant: true, neighbor: true, wantErr: model.ErrForbidden},
		{name: "unknown counterpart", actorParticipant: true, wantErr: model.ErrThreadNotFound},
		{name: "non-neighbor participant", actorParticipant: true, counterpartParticipant: true, wantErr: model.ErrThreadNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAccess(test.actorParticipant, test.counterpartParticipant, test.neighbor)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("validateAccess() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
