package repository

import (
	"errors"
	"testing"

	"swap-chain/modules/chat/model"
)

func TestNewPostgreSQLRequiresDatabase(t *testing.T) {
	if _, err := NewPostgreSQL(nil); err == nil {
		t.Fatal("NewPostgreSQL(nil) error = nil")
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
