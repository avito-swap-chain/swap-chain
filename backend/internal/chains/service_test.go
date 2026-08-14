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
