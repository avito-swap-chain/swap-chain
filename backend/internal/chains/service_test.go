package chains

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   CreateInput
		wantIDs []int64
		wantErr bool
	}{
		{
			name: "two item cycle",
			input: CreateInput{Edges: []Edge{
				{SourceItemID: 2, TargetItemID: 1},
				{SourceItemID: 1, TargetItemID: 2},
			}},
			wantIDs: []int64{1, 2},
		},
		{
			name: "three item cycle",
			input: CreateInput{Edges: []Edge{
				{SourceItemID: 1, TargetItemID: 2},
				{SourceItemID: 2, TargetItemID: 3},
				{SourceItemID: 3, TargetItemID: 1},
			}},
			wantIDs: []int64{1, 2, 3},
		},
		{
			name: "open path",
			input: CreateInput{Edges: []Edge{
				{SourceItemID: 1, TargetItemID: 2},
				{SourceItemID: 2, TargetItemID: 3},
			}},
			wantErr: true,
		},
		{
			name: "duplicate source",
			input: CreateInput{Edges: []Edge{
				{SourceItemID: 1, TargetItemID: 2},
				{SourceItemID: 1, TargetItemID: 3},
			}},
			wantErr: true,
		},
		{
			name: "duplicate target",
			input: CreateInput{Edges: []Edge{
				{SourceItemID: 1, TargetItemID: 3},
				{SourceItemID: 2, TargetItemID: 3},
			}},
			wantErr: true,
		},
		{
			name: "self edge",
			input: CreateInput{Edges: []Edge{
				{SourceItemID: 1, TargetItemID: 1},
				{SourceItemID: 2, TargetItemID: 1},
			}},
			wantErr: true,
		},
		{
			name: "too long",
			input: CreateInput{Edges: []Edge{
				{SourceItemID: 1, TargetItemID: 2},
				{SourceItemID: 2, TargetItemID: 3},
				{SourceItemID: 3, TargetItemID: 4},
				{SourceItemID: 4, TargetItemID: 1},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, _, err := validateCreate(tt.input)
			var validationErr *ValidationError
			if tt.wantErr {
				if !errors.As(err, &validationErr) {
					t.Fatalf("validateCreate() error = %v, want ValidationError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateCreate() error = %v", err)
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) {
				t.Fatalf("item IDs = %v, want %v", ids, tt.wantIDs)
			}
		})
	}
}

func TestValidateDecision(t *testing.T) {
	t.Parallel()
	for _, decision := range []string{DecisionApproved, DecisionDeclined} {
		if err := validateDecision(decision); err != nil {
			t.Fatalf("validateDecision(%q) error = %v", decision, err)
		}
	}
	if err := validateDecision("MAYBE"); err == nil {
		t.Fatal("validateDecision(MAYBE) succeeded")
	}
}

func TestChainCollectionHelpers(t *testing.T) {
	t.Parallel()

	owners := map[int64]int64{30: 7, 10: 9, 20: 7}
	if !containsOwner(owners, 9) || containsOwner(owners, 8) {
		t.Fatalf("containsOwner() returned an inconsistent result for %v", owners)
	}
	if got, want := distinctOwnerIDs(owners), []int64{7, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("distinctOwnerIDs() = %v, want %v", got, want)
	}
	if got, want := sortedMapKeys(map[int64][]int64{8: nil, 2: nil, 5: nil}), []int64{2, 5, 8}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedMapKeys() = %v, want %v", got, want)
	}

	chain := Chain{Participants: []Participant{{User: User{ID: 3}}, {User: User{ID: 6}}}}
	if !hasParticipant(chain, 6) || hasParticipant(chain, 4) {
		t.Fatalf("hasParticipant() returned an inconsistent result for %+v", chain.Participants)
	}
}

func TestPostgresServicePublishesParticipants(t *testing.T) {
	t.Parallel()

	var gotUsers []int64
	var gotType, gotEntity string
	var gotData map[string]any
	svc := NewPostgresService(nil, func(users []int64, eventType, entityID string, data map[string]any) {
		gotUsers, gotType, gotEntity, gotData = users, eventType, entityID, data
	})
	chain := Chain{ID: 42, Participants: []Participant{{User: User{ID: 7}}, {User: User{ID: 9}}}}
	svc.notify(chain, "CHAIN_UPDATED", map[string]any{"status": StatusAccepted})

	if !reflect.DeepEqual(gotUsers, []int64{7, 9}) || gotType != "CHAIN_UPDATED" || gotEntity != "42" {
		t.Fatalf("published users/type/entity = %v/%q/%q", gotUsers, gotType, gotEntity)
	}
	if gotData["status"] != StatusAccepted {
		t.Fatalf("published data = %v", gotData)
	}
}
